package main

// 017 §8: keep the active session unchanged and save only the next-launch preference.
func saveWidescreenPreference(location settingsLocation, current settings, next bool) (settings, error) {
	updated := current
	updated.Widescreen = next
	if err := saveSettings(location.path, location.forSave(updated)); err != nil {
		return current, err
	}
	return updated, nil
}
