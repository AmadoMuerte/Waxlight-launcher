package tray

// Labels holds the localized tray menu strings.
type Labels struct{ Open, Quit string }

var labels = map[string]Labels{
	"en": {Open: "Open Waxlight", Quit: "Quit"},
	"ru": {Open: "Открыть Waxlight", Quit: "Выход"},
	"be": {Open: "Адкрыць Waxlight", Quit: "Выхад"},
	"es": {Open: "Abrir Waxlight", Quit: "Salir"},
	"fr": {Open: "Ouvrir Waxlight", Quit: "Quitter"},
	"de": {Open: "Waxlight öffnen", Quit: "Beenden"},
	"kk": {Open: "Waxlight-ты ашу", Quit: "Шығу"},
	"pl": {Open: "Otwórz Waxlight", Quit: "Zakończ"},
	"sv": {Open: "Öppna Waxlight", Quit: "Avsluta"},
	"pt": {Open: "Abrir Waxlight", Quit: "Sair"},
}

// LabelsFor returns menu labels for a language code, falling back to English.
func LabelsFor(lang string) Labels {
	if l, ok := labels[lang]; ok {
		return l
	}
	return labels["en"]
}
