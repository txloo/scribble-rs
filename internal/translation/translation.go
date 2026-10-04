// Package translation provides live chat translation via the Google Cloud
// Translation v2 REST API. Messages are translated lazily at delivery time
// into the requester's interface language; results are cached per message
// and language, so each text is only ever sent to the API once. If no API
// key is configured or the API fails, messages are delivered unchanged,
// so the chat never breaks.
package translation

import (
	json "encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/scribble-rs/scribble.rs/internal/game"
)

// defaultEndpoint is Google's language translate v2 endpoint.
const defaultEndpoint = "https://translation.googleapis.com/language/translate/v2"

// supportedTargets contains the only languages chat is ever translated
// into. Requests for anything else are not translated.
var supportedTargets = map[string]bool{
	"en": true,
	"es": true,
}

// requestTimeout keeps chat polls responsive even when the API is slow.
const requestTimeout = 2 * time.Second

// errorLogCooldown spaces out error logs, so a persistently broken API
// doesn't flood the log on every poll.
const errorLogCooldown = time.Minute

// Client translates chat messages into a target language, caching results.
// A nil-ish client (no API key) is valid and translates nothing.
type Client struct {
	apiKey   string
	endpoint string
	http     *http.Client

	mutex       sync.Mutex
	// cache maps "messageID|targetLanguage" to the translation result.
	// Bounded implicitly by the chat log capacity times the number of
	// supported targets.
	cache map[string]*cachedTranslation

	lastErrorLog time.Time
}

type cachedTranslation struct {
	text     string
	sourceLanguage string
}

// NewClient creates a translation client hitting the default Google
// endpoint. With an empty API key, the returned client is disabled and all
// functions behave as no-ops.
func NewClient(apiKey string) *Client {
	return NewClientWithEndpoint(apiKey, defaultEndpoint)
}

// NewClientWithEndpoint creates a translation client for a custom
// endpoint; used by tests to stub the API.
func NewClientWithEndpoint(apiKey, endpoint string) *Client {
	return &Client{
		apiKey:   apiKey,
		endpoint: endpoint,
		http:     &http.Client{Timeout: requestTimeout},
		cache:    make(map[string]*cachedTranslation),
	}
}

// Enabled reports whether translation is configured.
func (client *Client) Enabled() bool {
	return client.apiKey != ""
}

// FillTranslations attaches translations to the given messages for the
// target language, in place. The messages must be delivery copies, not
// the chat log's storage, since this function mutates them. Messages
// already in the target language are not translated.
func (client *Client) FillTranslations(messages []game.StoreMessage, targetLanguage string) {
	if client == nil || !client.Enabled() || !supportedTargets[targetLanguage] {
		return
	}

	client.mutex.Lock()
	defer client.mutex.Unlock()

	// Collect the unique texts that are not cached yet. The API charges
	// per character, so identical texts are only sent once.
	type pending struct {
		text      string
		indices   []int
	}
	pendingTexts := make(map[string]*pending)
	for index, message := range messages {
		key := cacheKey(message.ID, targetLanguage)
		if _, hit := client.cache[key]; hit {
			continue
		}

		entry, exists := pendingTexts[message.Content]
		if !exists {
			entry = &pending{text: message.Content}
			pendingTexts[message.Content] = entry
		}
		entry.indices = append(entry.indices, index)
	}

	if len(pendingTexts) == 0 {
		applyCachedTranslations(messages, targetLanguage, client.cache)
		return
	}

	texts := make([]string, 0, len(pendingTexts))
	for _, entry := range pendingTexts {
		texts = append(texts, entry.text)
	}

	translations, err := client.requestTranslations(texts, targetLanguage)
	if err != nil {
		client.logRateLimited(err)
		return
	}

	textIndex := 0
	for _, entry := range pendingTexts {
		translation := translations[textIndex]
		textIndex++

		// Messages already in the target language are not "translated";
		// showing the original twice is noise. They are still cached (with
		// an empty translation), so subsequent deliveries don't hit the
		// API again.
		if normalizeLanguage(translation.DetectedSourceLanguage) == targetLanguage {
			for _, index := range entry.indices {
				client.cache[cacheKey(messages[index].ID, targetLanguage)] = &cachedTranslation{
					text:           "",
					sourceLanguage: translation.DetectedSourceLanguage,
				}
			}
			continue
		}

		for _, index := range entry.indices {
			client.cache[cacheKey(messages[index].ID, targetLanguage)] = &cachedTranslation{
				text:           translation.Text,
				sourceLanguage: translation.DetectedSourceLanguage,
			}
		}
	}

	applyCachedTranslations(messages, targetLanguage, client.cache)
}

// applyCachedTranslations copies cached translations into the delivery
// copies. Callers must hold the client mutex.
func applyCachedTranslations(
	messages []game.StoreMessage,
	targetLanguage string,
	cache map[string]*cachedTranslation,
) {
	for index := range messages {
		key := cacheKey(messages[index].ID, targetLanguage)
		if cached, hit := cache[key]; hit {
			messages[index].TranslatedContent = cached.text
			messages[index].SourceLanguage = cached.sourceLanguage
		}
	}
}

func cacheKey(messageID uint64, targetLanguage string) string {
	return fmt.Sprintf("%d|%s", messageID, targetLanguage)
}

type translationResponse struct {
	Text                   string `json:"translatedText"`
	DetectedSourceLanguage string `json:"detectedSourceLanguage"`
}

type translationAPIResponse struct {
	Data struct {
		Translations []translationResponse `json:"translations"`
	} `json:"data"`
}

// requestTranslations performs one batched API call. The returned
// translations are ordered like the given texts. Callers must hold the
// client mutex.
func (client *Client) requestTranslations(texts []string, targetLanguage string) ([]translationResponse, error) {
	form := url.Values{}
	form.Set("target", targetLanguage)
	form.Set("format", "text")
	for _, text := range texts {
		form.Add("q", text)
	}

	request, err := http.NewRequest(http.MethodPost, client.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("error creating translation request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("X-Goog-Api-Key", client.apiKey)

	response, err := client.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error sending translation request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("translation API returned status %d", response.StatusCode)
	}

	var parsed translationAPIResponse
	if err := json.NewDecoder(response.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("error parsing translation response: %w", err)
	}

	translations := parsed.Data.Translations
	if len(translations) != len(texts) {
		return nil, fmt.Errorf("translation API returned %d translations for %d texts",
			len(translations), len(texts))
	}

	return translations, nil
}

// logRateLimited logs an error at most once per cooldown period. Callers
// must hold the client mutex.
func (client *Client) logRateLimited(err error) {
	now := time.Now()
	if now.Sub(client.lastErrorLog) < errorLogCooldown {
		return
	}
	client.lastErrorLog = now
	log.Printf("chat translation unavailable: %s\n", err)
}

// normalizeLanguage maps a Google detected language code to the form used
// for comparisons, i.e. the two letter code in lowercase.
func normalizeLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	if len(language) > 2 {
		language = language[:2]
	}
	return language
}
