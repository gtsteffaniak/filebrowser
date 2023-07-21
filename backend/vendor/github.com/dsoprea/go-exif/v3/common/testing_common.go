package exifcommon

import (
	"os"
	"path"

	"encoding/binary"
	"io/ioutil"

	"github.com/dsoprea/go-logging"
)

var (
	moduleRootPath = ""

	testExifData []byte = nil

	// EncodeDefaultByteOrder is the default byte-order for encoding operations.
	EncodeDefaultByteOrder = binary.BigEndian

	// Default byte order for tests.
	TestDefaultByteOrder = binary.BigEndian
)

func GetModuleRootPath() string {
	if moduleRootPath == "" {
		moduleRootPath = os.Getenv("EXIF_MODULE_ROOT_PATH")
		if moduleRootPath != "" {
			return moduleRootPath
		}

		currentWd, err := os.Getwd()
		log.PanicIf(err)

		currentPath := currentWd

		visited := make([]string, 0)

		for {
			tryStampFilepath := path.Join(currentPath, ".MODULE_ROOT")

			_, err := os.Stat(tryStampFilepath)
			if err != nil && os.IsNotExist(err) != true {
				log.Panic(err)
			} else if err == nil {
				break
			}

			visited = append(visited, tryStampFilepath)

			currentPath = path.Dir(currentPath)
			if currentPath == "/" {
				log.Panicf("could not find module-root: %v", visited)
			}
		}

		moduleRootPath = currentPath
	}

	return moduleRootPath
}

func GetTestAssetsPath() string {
	moduleRootPath := GetModuleRootPath()
	assetsPath := path.Join(moduleRootPath, "assets")

	return assetsPath
}

func getTestImageFilepath() string {
	assetsPath := GetTestAssetsPath()
	testImageFilepath := path.Join(assetsPath, "NDM_8901.jpg")
	return testImageFilepath
}

func getTestExifData() []byte {
	if testExifData == nil {
		assetsPath := GetTestAssetsPath()
		filepath := path.Join(assetsPath, "NDM_8901.jpg.exif")

		var err error

		testExifData, err = ioutil.ReadFile(filepath)
		log.PanicIf(err)
	}

	return testExifData
}
