package translations

func initSpainTranslation() {
	translation := createTranslation()

	translation.put("requires-js", "Este sitio web requiere JavaScript para funcionar correctamente.")
	translation.put("click-to-homepage", "Haz clic aquí para volver a la página de inicio")

	translation.put("start-the-game", "¡Prepárate!")
	translation.put("force-start", "Inicio forzado")
	translation.put("force-restart", "Reinicio forzado")
	translation.put("game-not-started-title", "El juego no ha comenzado")
	translation.put("waiting-for-host-to-start", "Por favor, espera a que el anfitrión de tu lobby inicie el juego.")

	translation.put("now-spectating-title", "Ahora estás observando")
	translation.put("now-spectating-text", "Puedes salir del modo espectador pulsando el botón del ojo de la parte superior.")
	translation.put("now-participating-title", "Ahora estás participando")
	translation.put("now-participating-text", "Puedes entrar en el modo espectador pulsando el botón del ojo de la parte superior.")

	translation.put("spectation-requested-title", "Modo espectador solicitado")
	translation.put("spectation-requested-text", "Serás espectador cuando acabe este turno.")
	translation.put("participation-requested-title", "Participación solicitada")
	translation.put("participation-requested-text", "Participarás cuando acabe este turno.")

	translation.put("spectation-request-cancelled-title", "Solicitud de modo espectador cancelada")
	translation.put("spectation-request-cancelled-text", "Tu solicitud de espectador ha sido cancelada, seguirás participando.")
	translation.put("participation-request-cancelled-title", "Solicitud de participación cancelada")
	translation.put("participation-request-cancelled-text", "Tu solicitud de participación ha sido cancelada, seguirás observando.")

	translation.put("round", "Ronda")
	translation.put("toggle-soundeffects", "Activar o desactivar los efectos de sonido")
	translation.put("toggle-pen-pressure", "Activar o desactivar la presión del lápiz")
	translation.put("change-your-name", "Apodo")
	translation.put("randomize", "Aleatorizar")
	translation.put("apply", "Aplicar")
	translation.put("save", "Guardar")
	translation.put("toggle-fullscreen", "Alternar pantalla completa")
	translation.put("toggle-spectate", "Activar o desactivar el modo espectador")
	translation.put("show-help", "Mostrar ayuda")
	translation.put("votekick-a-player", "Votar para expulsar a un jugador")

	translation.put("last-turn", "(Última ronda: %s)")

	translation.put("drawer-kicked", "Como el jugador expulsado estaba dibujando, ninguno de vosotros obtendrá puntos en esta ronda.")
	translation.put("self-kicked", "Te han expulsado")
	translation.put("self-kicked-text", "Te han expulsado del lobby.")
	translation.put("kick-vote", "(%s/%s) jugadores votaron para expulsar a %s.")
	translation.put("player-kicked", "El jugador ha sido expulsado.")
	translation.put("owner-change", "%s es el nuevo dueño del lobby.")

	translation.put("leave-lobby", "Salir del lobby")
	translation.put("left-the-lobby", "%s salió del lobby.")
	translation.put("close-lobby", "Cerrar la sala")
	translation.put("close-lobby-error", "No se pudo cerrar la sala. ¿Sigues siendo su dueño?")
	translation.put("lobby-closed-title", "Sala cerrada")
	translation.put("lobby-closed-text", "El dueño ha cerrado la sala.")

	translation.put("change-lobby-settings-tooltip", "Cambiar la configuración del lobby")
	translation.put("change-lobby-settings-title", "Configuración del lobby")
	translation.put("lobby-settings-changed", "Se cambió la configuración del lobby")
	translation.put("advanced-settings", "Configuración avanzada")
	translation.put("chill", "Relajado")
	translation.put("competitive", "Competitivo")
	translation.put("chill-alt", "Aunque acertar rápido da más puntos, no es grave si tardas un poco más.\nLa puntuación base es bastante alta; céntrate en divertirte.")
	translation.put("competitive-alt", "Cuanto más rápido aciertes, más puntos obtendrás.\nLa puntuación base es mucho más baja y la bajada es más rápida.")
	translation.put("score-calculation", "Puntuación")
	translation.put("ui-language", "Idioma")
	translation.put("word-language", "Idioma de las palabras")
	translation.put("drawing-time-setting", "Tiempo de dibujo")
	translation.put("rounds-setting", "Rondas")
	translation.put("max-players-setting", "Máximo de jugadores")
	translation.put("public-lobby-setting", "Lobby público")
	translation.put("custom-words", "Palabras personalizadas")
	translation.put("custom-words-info", "Introduce tus palabras adicionales, separadas por comas")
	translation.put("custom-words-placeholder", "Lista de palabras, separadas, por comas")
	translation.put("custom-words-per-turn-setting", "Palabras personalizadas por turno")
	translation.put("players-per-ip-limit-setting", "Jugadores por límite de IP")
	translation.put("words-per-turn-setting", "Palabras por turno")
	translation.put("save-settings", "Guardar configuración")
	translation.put("input-contains-invalid-data", "Tu entrada contiene datos no válidos:")
	translation.put("please-fix-invalid-input", "Corrige la entrada no válida y vuelve a intentarlo.")
	translation.put("create-lobby", "Crear lobby")
	translation.put("create-public-lobby", "Crear un lobby público")
	translation.put("create-private-lobby", "Crear un lobby privado")
	translation.put("no-lobbies-yet", "Aún no hay lobbies.")
	translation.put("lobby-full", "Lo sentimos, pero el lobby está lleno.")
	translation.put("lobby-ip-limit-excceeded", "Lo sentimos, pero has superado el número máximo de clientes por IP.")
	translation.put("lobby-open-tab-exists", "Parece que ya tienes una pestaña abierta para este lobby.")
	translation.put("lobby-doesnt-exist", "El lobby solicitado no existe")

	translation.put("refresh", "Actualizar")
	translation.put("join-lobby", "Unirse al lobby")
	translation.put("join", "Unirse")
	translation.put("ready", "Listo")
	translation.put("ongoing", "En curso")
	translation.put("game-over-lobby", "Fin del juego")

	translation.put("message-input-placeholder", "Escribe tus conjeturas y mensajes aquí")

	translation.put("word-choice-warning", "Palabra si no eliges a tiempo")
	translation.put("choose-a-word", "Elige una palabra")
	translation.put("waiting-for-word-selection", "Esperando la selección de palabras")
	// This one doesn't use %s, since we want to make one part bold.
	translation.put("is-choosing-word", "está eligiendo una palabra.")

	translation.put("close-guess", "«%s» está muy cerca.")
	translation.put("correct-guess", "Has adivinado correctamente la palabra.")
	translation.put("correct-guess-other-player", "«%s» adivinó correctamente la palabra.")
	translation.put("round-over", "Turno terminado, no se eligió ninguna palabra.")
	translation.put("round-over-no-word", "Turno terminado, la palabra era «%s».")
	translation.put("game-over-win", "¡Enhorabuena, has ganado!")
	translation.put("game-over-tie", "¡Es un empate!")
	translation.put("game-over", "Quedaste en el puesto %s con %s puntos")
	translation.put("drawer-disconnected", "El turno terminó antes de tiempo; quien dibujaba se desconectó.")
	translation.put("guessers-disconnected", "El turno terminó antes de tiempo; los jugadores que adivinaban se desconectaron.")
	translation.put("word-hint-revealed", "¡Se ha revelado una pista de la palabra!")

	translation.put("change-active-color", "Cambia tu color activo")
	translation.put("use-pencil", "Usa el lápiz")
	translation.put("use-eraser", "Usa el borrador")
	translation.put("use-fill-bucket", "Usa el cubo de relleno (Rellena el área objetivo con el color seleccionado)")
	translation.put("change-pencil-size-to", "Cambia el tamaño del lápiz o del borrador a %s")
	translation.put("clear-canvas", "Limpiar el lienzo")
	translation.put("undo", "Deshacer el último cambio (No funciona después de «"+translation.Get("clear-canvas")+"»)")
	translation.put("undo-help-message", "Deshacer")

	translation.put("connection-lost", "¡Conexión perdida!")
	translation.put("connection-lost-text", "Intentando reconectar"+
		" ...\n\nAsegúrate de que tu conexión a Internet funcione.\n"+
		"Si el problema persiste, contacta con el administrador.")
	translation.put("error-connecting", "Error al conectar con el servidor")
	translation.put("error-connecting-text",
		"Scribble.rs no pudo establecer una conexión de socket.\n\nAunque tu conexión "+
			"a Internet parece funcionar, es posible que el servidor\no tu cortafuegos no se hayan "+
			"configurado correctamente.\n\nPara reintentarlo, recarga la página.")
	translation.put("message-too-long", "Tu mensaje es demasiado largo.")
	translation.put("server-shutting-down-title", "Apagando el servidor")
	translation.put("server-shutting-down-text", "Lo sentimos, pero el servidor está a punto de apagarse. Vuelve más tarde.")

	// Help dialog
	translation.put("controls", "Controles")
	translation.put("pencil", "Lápiz")
	translation.put("eraser", "Borrador")
	translation.put("fill-bucket", "Cubo de relleno")
	// No %s placeholders here, as the keyhints are appended by the client.
	translation.put("switch-pencil-sizes", "También puedes cambiar el tamaño de las herramientas con las teclas")
	translation.put("forbidden", "Prohibido")

	// Generic words
	// "close" as in "closing the window"
	translation.put("close", "Cerrar")
	translation.put("no", "No")
	translation.put("yes", "Sí")
	translation.put("system", "Sistema")
	translation.put("confirm", "Aceptar")

	translation.put("source-code", "Código fuente")
	translation.put("help", "Ayuda")
	translation.put("submit-feedback", "Comentarios")
	translation.put("stats", "Estadísticas")

	RegisterTranslation("es", translation)
}
