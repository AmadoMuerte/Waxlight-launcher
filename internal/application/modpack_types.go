package application

import vsmodpack "github.com/AmadoMuerte/vintagestory-go/modpack"

// ModUpdateReport is kept at the application boundary; presentation converts
// it to the stable Wails DTO without exposing provider types to the frontend.
type ModUpdateReport = vsmodpack.Report
