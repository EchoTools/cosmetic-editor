//go:build !android

package main

import (
	_ "embed"
	"os"
	"path/filepath"
)

// The Blender model import (chassis, bracers, boosters) runs these scripts
// with the user's Python and Blender. They are PC only, so the Android build
// leaves them out rather than carry them in the APK.

//go:embed Data/backend_chassis_builder.py
var embeddedBackendChassisBuilder []byte

//go:embed Data/blender_chassis_processor.py
var embeddedBlenderChassisProcessor []byte

//go:embed Data/evr_mesh_importer.zip
var embeddedEvrMeshImporter []byte

// installModelScripts writes the model import scripts to Temp/Scripts, where
// the chassis, bracers and boosters editors run them from.
func installModelScripts(tempDir string) {
	scriptsDir := filepath.Join(tempDir, "Scripts")
	os.MkdirAll(scriptsDir, 0755)
	os.WriteFile(filepath.Join(scriptsDir, "backend_chassis_builder.py"), embeddedBackendChassisBuilder, 0644)
	os.WriteFile(filepath.Join(scriptsDir, "blender_chassis_processor.py"), embeddedBlenderChassisProcessor, 0644)
	os.WriteFile(filepath.Join(scriptsDir, "evr_mesh_importer.zip"), embeddedEvrMeshImporter, 0644)
}
