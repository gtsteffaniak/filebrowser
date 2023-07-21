package exif

import (
	"path"
	"reflect"
	"testing"

	"io/ioutil"

	"github.com/dsoprea/go-logging"

	"github.com/dsoprea/go-exif/v3/common"
)

var (
	testExifData []byte
)

func getExifSimpleTestIb() *IfdBuilder {
	defer func() {
		if state := recover(); state != nil {
			err := log.Wrap(state.(error))
			log.Panic(err)
		}
	}()

	im := exifcommon.NewIfdMapping()

	err := exifcommon.LoadStandardIfds(im)
	log.PanicIf(err)

	ti := NewTagIndex()
	ib := NewIfdBuilder(im, ti, exifcommon.IfdStandardIfdIdentity, exifcommon.TestDefaultByteOrder)

	err = ib.AddStandard(0x000b, "asciivalue")
	log.PanicIf(err)

	err = ib.AddStandard(0x00ff, []uint16{0x1122})
	log.PanicIf(err)

	err = ib.AddStandard(0x0100, []uint32{0x33445566})
	log.PanicIf(err)

	err = ib.AddStandard(0x013e, []exifcommon.Rational{{Numerator: 0x11112222, Denominator: 0x33334444}})
	log.PanicIf(err)

	return ib
}

func getExifSimpleTestIbBytes() []byte {
	defer func() {
		if state := recover(); state != nil {
			err := log.Wrap(state.(error))
			log.Panic(err)
		}
	}()

	im := exifcommon.NewIfdMapping()

	err := exifcommon.LoadStandardIfds(im)
	log.PanicIf(err)

	ti := NewTagIndex()
	ib := NewIfdBuilder(im, ti, exifcommon.IfdStandardIfdIdentity, exifcommon.TestDefaultByteOrder)

	err = ib.AddStandard(0x000b, "asciivalue")
	log.PanicIf(err)

	err = ib.AddStandard(0x00ff, []uint16{0x1122})
	log.PanicIf(err)

	err = ib.AddStandard(0x0100, []uint32{0x33445566})
	log.PanicIf(err)

	err = ib.AddStandard(0x013e, []exifcommon.Rational{{Numerator: 0x11112222, Denominator: 0x33334444}})
	log.PanicIf(err)

	ibe := NewIfdByteEncoder()

	exifData, err := ibe.EncodeToExif(ib)
	log.PanicIf(err)

	return exifData
}

func validateExifSimpleTestIb(exifData []byte, t *testing.T) {
	defer func() {
		if state := recover(); state != nil {
			err := log.Wrap(state.(error))
			log.Panic(err)
		}
	}()

	im := exifcommon.NewIfdMapping()

	err := exifcommon.LoadStandardIfds(im)
	log.PanicIf(err)

	ti := NewTagIndex()

	eh, index, err := Collect(im, ti, exifData)
	log.PanicIf(err)

	if eh.ByteOrder != exifcommon.TestDefaultByteOrder {
		t.Fatalf("EXIF byte-order is not correct: %v", eh.ByteOrder)
	} else if eh.FirstIfdOffset != ExifDefaultFirstIfdOffset {
		t.Fatalf("EXIF first IFD-offset not correct: (0x%02x)", eh.FirstIfdOffset)
	}

	if len(index.Ifds) != 1 {
		t.Fatalf("There wasn't exactly one IFD decoded: (%d)", len(index.Ifds))
	}

	ifd := index.RootIfd

	if ifd.ByteOrder() != exifcommon.TestDefaultByteOrder {
		t.Fatalf("IFD byte-order not correct.")
	} else if ifd.ifdIdentity.UnindexedString() != exifcommon.IfdStandardIfdIdentity.UnindexedString() {
		t.Fatalf("IFD name not correct.")
	} else if ifd.ifdIdentity.Index() != 0 {
		t.Fatalf("IFD index not zero: (%d)", ifd.ifdIdentity.Index())
	} else if ifd.Offset() != uint32(0x0008) {
		t.Fatalf("IFD offset not correct.")
	} else if len(ifd.Entries()) != 4 {
		t.Fatalf("IFD number of entries not correct: (%d)", len(ifd.Entries()))
	} else if ifd.nextIfdOffset != uint32(0) {
		t.Fatalf("Next-IFD offset is non-zero.")
	} else if ifd.nextIfd != nil {
		t.Fatalf("Next-IFD pointer is non-nil.")
	}

	// Verify the values by using the actual, original types (this is awesome).

	expected := []struct {
		tagId uint16
		value interface{}
	}{
		{tagId: 0x000b, value: "asciivalue"},
		{tagId: 0x00ff, value: []uint16{0x1122}},
		{tagId: 0x0100, value: []uint32{0x33445566}},
		{tagId: 0x013e, value: []exifcommon.Rational{{Numerator: 0x11112222, Denominator: 0x33334444}}},
	}

	for i, ite := range ifd.Entries() {
		if ite.TagId() != expected[i].tagId {
			t.Fatalf("Tag-ID for entry (%d) not correct: (0x%02x) != (0x%02x)", i, ite.TagId(), expected[i].tagId)
		}

		value, err := ite.Value()
		log.PanicIf(err)

		if reflect.DeepEqual(value, expected[i].value) != true {
			t.Fatalf("Value for entry (%d) not correct: [%v] != [%v]", i, value, expected[i].value)
		}
	}
}

func getTestImageFilepath() string {
	assetsPath := exifcommon.GetTestAssetsPath()
	testImageFilepath := path.Join(assetsPath, "NDM_8901.jpg")
	return testImageFilepath
}

func getTestExifData() []byte {
	if testExifData == nil {
		assetsPath := exifcommon.GetTestAssetsPath()
		filepath := path.Join(assetsPath, "NDM_8901.jpg.exif")

		var err error

		testExifData, err = ioutil.ReadFile(filepath)
		log.PanicIf(err)
	}

	return testExifData
}

func getTestGpsImageFilepath() string {
	assetsPath := exifcommon.GetTestAssetsPath()
	testGpsImageFilepath := path.Join(assetsPath, "gps.jpg")
	return testGpsImageFilepath
}

func getTestGeotiffFilepath() string {
	assetsPath := exifcommon.GetTestAssetsPath()
	testGeotiffFilepath := path.Join(assetsPath, "geotiff_example.tif")
	return testGeotiffFilepath
}
