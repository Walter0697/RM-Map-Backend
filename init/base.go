package init

func InitDatabaseValue() error {
	if err := SeedAllTrainStation(); err != nil {
		return err
	}
	return nil
}
