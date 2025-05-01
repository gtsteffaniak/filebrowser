package cmd

// healthcheck attempt to save test file against configured url
func validateOfficeIntegration() {
	if settings.Config.Integrations.OnlyOffice.Url != "" {
		// get url health

		logger.Warning("Could not connect to only office Url, make sure its valid and running")
	}
}