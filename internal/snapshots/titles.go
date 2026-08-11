package snapshots

// Operation title i18n keys. The backend stores the English title as a
// fallback together with a translation key and its parameters; the frontend
// renders the title through its i18n system using these keys.
const (
	TitleCreatingSnapshot      = "operation_creating_snapshot"
	TitleCreatingSafetyBackup  = "operation_creating_safety_backup"
	TitleRestoringSnapshot     = "operation_restoring_snapshot"
	TitleRestoringFiles        = "operation_restoring_files"
	TitleDownloadingMods       = "operation_downloading_mods"
	TitleFinishingRestore      = "operation_finishing_restore"
	TitleRestoringModsProgress = "operation_restoring_mods_progress"
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
