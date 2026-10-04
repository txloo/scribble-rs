package frontend

import (
	"log"
	"net/http"

	"github.com/scribble-rs/scribble.rs/internal/api"
	"github.com/scribble-rs/scribble.rs/internal/state"
	"github.com/scribble-rs/scribble.rs/internal/translations"
)

// This file contains the home page: a permanent drawing wall with a shared
// chat. It is fully socketless — everything runs over the /v1/hub/* HTTP
// endpoints, so visiting the home page never requires a websocket.

// homeJsData defines the data that home.js requires. Since the CSP forbids
// inline scripts, server side data is baked into the file itself.
type homeJsData struct {
	*BasePageConfig
	*api.GameConstants

	Translation *translations.Translation
}

// homeJs serves the home page's javascript, templated with server side data,
// exactly like the lobby's lobby.js.
func (handler *SSRHandler) homeJs(writer http.ResponseWriter, request *http.Request) {
	translation, _ := determineTranslation(request)
	pageData := &homeJsData{
		BasePageConfig: handler.basePageConfig,
		GameConstants:  api.GameConstantsData,
		Translation:    translation,
	}

	writer.Header().Set("Content-Type", "text/javascript")
	// Duration of 1 year, since we use cachebusting anyway.
	writer.Header().Set("Cache-Control", "public, max-age=31536000")
	writer.WriteHeader(http.StatusOK)
	if err := handler.homeJsRawTemplate.ExecuteTemplate(writer, "home-js", pageData); err != nil {
		log.Printf("error templating JS: %s\n", err)
	}
}

// homePageData defines all non-static data for the home page.
type homePageData struct {
	*BasePageConfig
	*api.GameConstants

	Translation *translations.Translation
	Locale      string
	// UILanguages powers the interface language switcher in the chat bar.
	UILanguages []translations.UILanguage
}

// homePageHandler serves the home page. Opening the page places the visitor
// in the room right away: no joining step, no lobby UI, the URL stays the
// plain home path. The room is addressed implicitly by all endpoints.
func (handler *SSRHandler) homePageHandler(writer http.ResponseWriter, request *http.Request) {
	translation, locale := determineTranslation(request)

	hub := state.HubLobby()
	if hub == nil {
		// Only reachable in tests or misconfigured deployments.
		handler.indexPageHandler(writer, request)
		return
	}

	requestAddress := api.GetIPAddressFromRequest(request)

	var pageData *homePageData
	hub.Synchronized(func() {
		player := api.GetPlayer(hub, request)
		if player == nil {
			if !hub.HasFreePlayerSlot() {
				handler.userFacingError(writer, translation.Get("lobby-full"), translation)
				return
			}

			if !hub.CanIPConnect(requestAddress) {
				handler.userFacingError(writer, translation.Get("lobby-ip-limit-excceeded"), translation)
				return
			}

			player = hub.JoinPlayer(api.GetPlayername(request))
		}

		player.SetLastKnownAddress(requestAddress)
		player.TouchLastSeen()
		api.SetGameplayCookies(writer, request, player, hub)

		pageData = &homePageData{
			BasePageConfig: handler.basePageConfig,
			GameConstants:  api.GameConstantsData,
			Translation:    translation,
			Locale:         locale,
			UILanguages:    translations.GetUILanguages(),
		}
	})

	// If the pageData isn't initialized, an error has occurred and has
	// already been handled.
	if pageData != nil {
		if err := pageTemplates.ExecuteTemplate(writer, "home-page", pageData); err != nil {
			log.Printf("Error templating home page: %s\n", err)
		}
	}
}
