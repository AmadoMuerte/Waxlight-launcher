package application

// Operation title i18n keys. The backend stores the English title as a
// fallback together with a translation key and its parameters; the frontend
// renders the title through its i18n system using these keys.
const (
	operationTitleInstallingGameVersion  = "operation_installing_game_version"
	operationTitleDownloadingGameVersion = "operation_downloading_game_version"
	operationTitleInstallingMod          = "operation_installing_mod"
	operationTitleCreatingSnapshot       = "operation_creating_snapshot"
	operationTitleRestoringSnapshot      = "operation_restoring_snapshot"
	operationTitleRestoringFiles         = "operation_restoring_files"
	operationTitleDownloadingMods        = "operation_downloading_mods"
	operationTitleFinishingRestore       = "operation_finishing_restore"
	operationTitleRestoringModsProgress  = "operation_restoring_mods_progress"
)

// titleParams builds the interpolation parameters for a localized operation
// title. Values are plain, non-secret display data.
func titleParams(values ...string) map[string]string {
	params := make(map[string]string, len(values)/2)
	for index := 0; index+1 < len(values); index += 2 {
		params[values[index]] = values[index+1]
	}
	return params
}
