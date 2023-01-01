package initdb

func InitDatabaseValue() error {
	// appending marker country field
	if err := AppendMarkerCountryColumn(); err != nil {
		return err
	}

	// seeding for the train station
	if err := SeedAllTrainStation(); err != nil {
		return err
	}
	return nil
}
