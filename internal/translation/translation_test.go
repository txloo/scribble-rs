package translation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scribble-rs/scribble.rs/internal/game"
	"github.com/stretchr/testify/require"
)

func stubMessages() []game.StoreMessage {
	return []game.StoreMessage{
		{OutgoingMessage: game.OutgoingMessage{Content: "hello world"}, ID: 1},
		{OutgoingMessage: game.OutgoingMessage{Content: "hello world"}, ID: 2},
		{OutgoingMessage: game.OutgoingMessage{Content: "guten tag"}, ID: 3},
	}
}

func stubAPIResponse(translations []translationResponse) translationAPIResponse {
	var response translationAPIResponse
	response.Data.Translations = translations
	return response
}

//nolint:paralleltest //this test is very stateful
func TestDisabledClientIsNoOp(t *testing.T) {
	client := NewClient("")
	messages := stubMessages()

	client.FillTranslations(messages, "es")

	require.Empty(t, messages[0].TranslatedContent)
	require.Empty(t, messages[0].SourceLanguage)
}

//nolint:paralleltest //this test is very stateful
func TestUnsupportedTargetIsNoOp(t *testing.T) {
	client := NewClient("test-key")
	messages := stubMessages()

	client.FillTranslations(messages, "fr")

	require.Empty(t, messages[0].TranslatedContent)
}

//nolint:paralleltest //this test is very stateful
func TestTranslationAndCache(t *testing.T) {
	var apiCalls int

	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		apiCalls++
		require.NoError(t, request.ParseForm())
		// Identical texts must be deduplicated into one q entry.
		require.Equal(t, []string{"hello world", "guten tag"}, request.Form["q"])
		require.Equal(t, "es", request.Form.Get("target"))

		writer.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(writer).Encode(stubAPIResponse([]translationResponse{
			{TranslatedText: "hola mundo", DetectedSourceLanguage: "en"},
			{TranslatedText: "guten tag", DetectedSourceLanguage: "de"},
		})))
	}))
	defer testServer.Close()

	client := NewClientWithEndpoint("test-key", testServer.URL)

	messages := stubMessages()
	client.FillTranslations(messages, "es")

	require.Equal(t, "hola mundo", messages[0].TranslatedContent)
	require.Equal(t, "en", messages[0].SourceLanguage)
	// The identical text shares the translation.
	require.Equal(t, "hola mundo", messages[1].TranslatedContent)
	// The German text is translated too.
	require.Equal(t, "guten tag", messages[2].TranslatedContent)
	require.Equal(t, 1, apiCalls, "One batched call must cover all texts.")

	// A second delivery of the same messages hits the cache only.
	client.FillTranslations(messages, "es")
	require.Equal(t, 1, apiCalls, "Cached translations must not trigger API calls.")
}

//nolint:paralleltest //this test is very stateful
func TestSameLanguageMessageIsNotTranslated(t *testing.T) {
	var apiCalls int

	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		apiCalls++
		require.NoError(t, request.ParseForm())
		writer.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(writer).Encode(stubAPIResponse([]translationResponse{
			{TranslatedText: "hola", DetectedSourceLanguage: "es"},
		})))
	}))
	defer testServer.Close()

	client := NewClientWithEndpoint("test-key", testServer.URL)

	messages := []game.StoreMessage{
		{OutgoingMessage: game.OutgoingMessage{Content: "hola"}, ID: 1},
	}
	client.FillTranslations(messages, "es")

	require.Empty(t, messages[0].TranslatedContent,
		"A message already in the target language must not be shown twice.")

	// The same-language message is cached as processed; a second delivery
	// must not call the API again.
	client.FillTranslations(messages, "es")
	require.Equal(t, 1, apiCalls,
		"Same-language messages must be cached to avoid repeated API calls.")
}

//nolint:paralleltest //this test is very stateful
func TestAPIFailureKeepsOriginals(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer testServer.Close()

	client := NewClientWithEndpoint("test-key", testServer.URL)

	messages := stubMessages()
	client.FillTranslations(messages, "es")

	for _, message := range messages {
		require.Empty(t, message.TranslatedContent,
			"On API failure the original message must be delivered unchanged.")
	}
}
