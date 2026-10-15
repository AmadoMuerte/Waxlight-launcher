package tray

import "testing"

func TestLabelsFor(t *testing.T) {
	tests := []struct {
		lang string
		want Labels
	}{
		{"en", Labels{Open: "Open Waxlight", Quit: "Quit"}},
		{"ru", Labels{Open: "Открыть Waxlight", Quit: "Выход"}},
		{"be", Labels{Open: "Адкрыць Waxlight", Quit: "Выхад"}},
		{"es", Labels{Open: "Abrir Waxlight", Quit: "Salir"}},
		{"fr", Labels{Open: "Ouvrir Waxlight", Quit: "Quitter"}},
		{"de", Labels{Open: "Waxlight öffnen", Quit: "Beenden"}},
		{"kk", Labels{Open: "Waxlight-ты ашу", Quit: "Шығу"}},
		{"pl", Labels{Open: "Otwórz Waxlight", Quit: "Zakończ"}},
		{"sv", Labels{Open: "Öppna Waxlight", Quit: "Avsluta"}},
		{"pt", Labels{Open: "Abrir Waxlight", Quit: "Sair"}},
		{"xx", Labels{Open: "Open Waxlight", Quit: "Quit"}},
		{"", Labels{Open: "Open Waxlight", Quit: "Quit"}},
	}
	for _, tt := range tests {
		if got := LabelsFor(tt.lang); got != tt.want {
			t.Errorf("LabelsFor(%q) = %+v, want %+v", tt.lang, got, tt.want)
		}
	}
}
