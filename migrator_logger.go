package flam

type MigratorLogger interface {
	LogUpStart(info MigrationInfo)
	LogUpError(info MigrationInfo, e error)
	LogUpDone(info MigrationInfo)

	LogDownStart(info MigrationInfo)
	LogDownError(info MigrationInfo, e error)
	LogDownDone(info MigrationInfo)
}
