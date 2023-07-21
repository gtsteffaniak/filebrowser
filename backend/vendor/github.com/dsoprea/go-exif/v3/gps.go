package exif

import (
	"errors"
	"fmt"
	"time"

	"github.com/dsoprea/go-logging"
	"github.com/golang/geo/s2"

	"github.com/dsoprea/go-exif/v3/common"
)

var (
	// ErrGpsCoordinatesNotValid means that some part of the geographic data was
	// unparseable.
	ErrGpsCoordinatesNotValid = errors.New("GPS coordinates not valid")
)

// GpsDegrees is a high-level struct representing geographic data.
type GpsDegrees struct {
	// Orientation describes the N/E/S/W direction that this position is
	// relative to.
	Orientation byte

	// Degrees is a simple float representing the underlying rational degrees
	// amount.
	Degrees float64

	// Minutes is a simple float representing the underlying rational minutes
	// amount.
	Minutes float64

	// Seconds is a simple float representing the underlying ration seconds
	// amount.
	Seconds float64
}

// NewGpsDegreesFromRationals returns a GpsDegrees struct given the EXIF-encoded
// information. The refValue is the N/E/S/W direction that this position is
// relative to.
func NewGpsDegreesFromRationals(refValue string, rawCoordinate []exifcommon.Rational) (gd GpsDegrees, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	if len(rawCoordinate) != 3 {
		log.Panicf("new GpsDegrees struct requires a raw-coordinate with exactly three rationals")
	}

	gd = GpsDegrees{
		Orientation: refValue[0],
		Degrees:     float64(rawCoordinate[0].Numerator) / float64(rawCoordinate[0].Denominator),
		Minutes:     float64(rawCoordinate[1].Numerator) / float64(rawCoordinate[1].Denominator),
		Seconds:     float64(rawCoordinate[2].Numerator) / float64(rawCoordinate[2].Denominator),
	}

	return gd, nil
}

// String provides returns a descriptive string.
func (d GpsDegrees) String() string {
	return fmt.Sprintf("Degrees<O=[%s] D=(%g) M=(%g) S=(%g)>", string([]byte{d.Orientation}), d.Degrees, d.Minutes, d.Seconds)
}

// Decimal calculates and returns the simplified float representation of the
// component degrees.
func (d GpsDegrees) Decimal() float64 {
	decimal := float64(d.Degrees) + float64(d.Minutes)/60.0 + float64(d.Seconds)/3600.0

	if d.Orientation == 'S' || d.Orientation == 'W' {
		return -decimal
	}

	return decimal
}

// Raw returns a Rational struct that can be used to *write* coordinates. In
// practice, the denominator are typically (1) in the original EXIF data, and,
// that being the case, this will best preserve precision.
func (d GpsDegrees) Raw() []exifcommon.Rational {
	return []exifcommon.Rational{
		{Numerator: uint32(d.Degrees), Denominator: 1},
		{Numerator: uint32(d.Minutes), Denominator: 1},
		{Numerator: uint32(d.Seconds), Denominator: 1},
	}
}

// GpsInfo encapsulates all of the geographic information in one place.
type GpsInfo struct {
	Latitude, Longitude GpsDegrees
	Altitude            int
	Timestamp           time.Time
}

// String returns a descriptive string.
func (gi *GpsInfo) String() string {
	return fmt.Sprintf("GpsInfo<LAT=(%.05f) LON=(%.05f) ALT=(%d) TIME=[%s]>",
		gi.Latitude.Decimal(), gi.Longitude.Decimal(), gi.Altitude, gi.Timestamp)
}

// S2CellId returns the cell-ID of the geographic location on the earth.
func (gi *GpsInfo) S2CellId() s2.CellID {
	latitude := gi.Latitude.Decimal()
	longitude := gi.Longitude.Decimal()

	ll := s2.LatLngFromDegrees(latitude, longitude)
	cellId := s2.CellIDFromLatLng(ll)

	if cellId.IsValid() == false {
		panic(ErrGpsCoordinatesNotValid)
	}

	return cellId
}
