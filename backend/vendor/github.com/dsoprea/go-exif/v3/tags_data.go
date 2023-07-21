package exif

var (
	// From assets/tags.yaml . Needs to be here so it's embedded in the binary.
	tagsYaml = `
# Notes:
#
# This file was produced from http://www.exiv2.org/tags.html, using the included
# tool, though that document appears to have some duplicates when all IDs are
# supposed to be unique (EXIF information only has IDs, not IFDs; IFDs are
# determined by our pre-existing knowledge of those tags).
#
# The webpage that we've produced this file from appears to indicate that
# ImageWidth is represented by both 0x0100 and 0x0001 depending on whether the
# encoding is RGB or YCbCr.
IFD/Exif:
- id: 0x829a
  name: ExposureTime
  type_name: RATIONAL
- id: 0x829d
  name: FNumber
  type_name: RATIONAL
- id: 0x8822
  name: ExposureProgram
  type_name: SHORT
- id: 0x8824
  name: SpectralSensitivity
  type_name: ASCII
- id: 0x8827
  name: ISOSpeedRatings
  type_name: SHORT
- id: 0x8828
  name: OECF
  type_name: UNDEFINED
- id: 0x8830
  name: SensitivityType
  type_name: SHORT
- id: 0x8831
  name: StandardOutputSensitivity
  type_name: LONG
- id: 0x8832
  name: RecommendedExposureIndex
  type_name: LONG
- id: 0x8833
  name: ISOSpeed
  type_name: LONG
- id: 0x8834
  name: ISOSpeedLatitudeyyy
  type_name: LONG
- id: 0x8835
  name: ISOSpeedLatitudezzz
  type_name: LONG
- id: 0x9000
  name: ExifVersion
  type_name: UNDEFINED
- id: 0x9003
  name: DateTimeOriginal
  type_name: ASCII
- id: 0x9004
  name: DateTimeDigitized
  type_name: ASCII
- id: 0x9010
  name: OffsetTime
  type_name: ASCII
- id: 0x9011
  name: OffsetTimeOriginal
  type_name: ASCII
- id: 0x9012
  name: OffsetTimeDigitized
  type_name: ASCII
- id: 0x9101
  name: ComponentsConfiguration
  type_name: UNDEFINED
- id: 0x9102
  name: CompressedBitsPerPixel
  type_name: RATIONAL
- id: 0x9201
  name: ShutterSpeedValue
  type_name: SRATIONAL
- id: 0x9202
  name: ApertureValue
  type_name: RATIONAL
- id: 0x9203
  name: BrightnessValue
  type_name: SRATIONAL
- id: 0x9204
  name: ExposureBiasValue
  type_name: SRATIONAL
- id: 0x9205
  name: MaxApertureValue
  type_name: RATIONAL
- id: 0x9206
  name: SubjectDistance
  type_name: RATIONAL
- id: 0x9207
  name: MeteringMode
  type_name: SHORT
- id: 0x9208
  name: LightSource
  type_name: SHORT
- id: 0x9209
  name: Flash
  type_name: SHORT
- id: 0x920a
  name: FocalLength
  type_name: RATIONAL
- id: 0x9214
  name: SubjectArea
  type_name: SHORT
- id: 0x927c
  name: MakerNote
  type_name: UNDEFINED
- id: 0x9286
  name: UserComment
  type_name: UNDEFINED
- id: 0x9290
  name: SubSecTime
  type_name: ASCII
- id: 0x9291
  name: SubSecTimeOriginal
  type_name: ASCII
- id: 0x9292
  name: SubSecTimeDigitized
  type_name: ASCII
- id: 0xa000
  name: FlashpixVersion
  type_name: UNDEFINED
- id: 0xa001
  name: ColorSpace
  type_name: SHORT
- id: 0xa002
  name: PixelXDimension
  type_names: [LONG, SHORT]
- id: 0xa003
  name: PixelYDimension
  type_names: [LONG, SHORT]
- id: 0xa004
  name: RelatedSoundFile
  type_name: ASCII
- id: 0xa005
  name: InteroperabilityTag
  type_name: LONG
- id: 0xa20b
  name: FlashEnergy
  type_name: RATIONAL
- id: 0xa20c
  name: SpatialFrequencyResponse
  type_name: UNDEFINED
- id: 0xa20e
  name: FocalPlaneXResolution
  type_name: RATIONAL
- id: 0xa20f
  name: FocalPlaneYResolution
  type_name: RATIONAL
- id: 0xa210
  name: FocalPlaneResolutionUnit
  type_name: SHORT
- id: 0xa214
  name: SubjectLocation
  type_name: SHORT
- id: 0xa215
  name: ExposureIndex
  type_name: RATIONAL
- id: 0xa217
  name: SensingMethod
  type_name: SHORT
- id: 0xa300
  name: FileSource
  type_name: UNDEFINED
- id: 0xa301
  name: SceneType
  type_name: UNDEFINED
- id: 0xa302
  name: CFAPattern
  type_name: UNDEFINED
- id: 0xa401
  name: CustomRendered
  type_name: SHORT
- id: 0xa402
  name: ExposureMode
  type_name: SHORT
- id: 0xa403
  name: WhiteBalance
  type_name: SHORT
- id: 0xa404
  name: DigitalZoomRatio
  type_name: RATIONAL
- id: 0xa405
  name: FocalLengthIn35mmFilm
  type_name: SHORT
- id: 0xa406
  name: SceneCaptureType
  type_name: SHORT
- id: 0xa407
  name: GainControl
  type_name: SHORT
- id: 0xa408
  name: Contrast
  type_name: SHORT
- id: 0xa409
  name: Saturation
  type_name: SHORT
- id: 0xa40a
  name: Sharpness
  type_name: SHORT
- id: 0xa40b
  name: DeviceSettingDescription
  type_name: UNDEFINED
- id: 0xa40c
  name: SubjectDistanceRange
  type_name: SHORT
- id: 0xa420
  name: ImageUniqueID
  type_name: ASCII
- id: 0xa430
  name: CameraOwnerName
  type_name: ASCII
- id: 0xa431
  name: BodySerialNumber
  type_name: ASCII
- id: 0xa432
  name: LensSpecification
  type_name: RATIONAL
- id: 0xa433
  name: LensMake
  type_name: ASCII
- id: 0xa434
  name: LensModel
  type_name: ASCII
- id: 0xa435
  name: LensSerialNumber
  type_name: ASCII
IFD/GPSInfo:
- id: 0x0000
  name: GPSVersionID
  type_name: BYTE
- id: 0x0001
  name: GPSLatitudeRef
  type_name: ASCII
- id: 0x0002
  name: GPSLatitude
  type_name: RATIONAL
- id: 0x0003
  name: GPSLongitudeRef
  type_name: ASCII
- id: 0x0004
  name: GPSLongitude
  type_name: RATIONAL
- id: 0x0005
  name: GPSAltitudeRef
  type_name: BYTE
- id: 0x0006
  name: GPSAltitude
  type_name: RATIONAL
- id: 0x0007
  name: GPSTimeStamp
  type_name: RATIONAL
- id: 0x0008
  name: GPSSatellites
  type_name: ASCII
- id: 0x0009
  name: GPSStatus
  type_name: ASCII
- id: 0x000a
  name: GPSMeasureMode
  type_name: ASCII
- id: 0x000b
  name: GPSDOP
  type_name: RATIONAL
- id: 0x000c
  name: GPSSpeedRef
  type_name: ASCII
- id: 0x000d
  name: GPSSpeed
  type_name: RATIONAL
- id: 0x000e
  name: GPSTrackRef
  type_name: ASCII
- id: 0x000f
  name: GPSTrack
  type_name: RATIONAL
- id: 0x0010
  name: GPSImgDirectionRef
  type_name: ASCII
- id: 0x0011
  name: GPSImgDirection
  type_name: RATIONAL
- id: 0x0012
  name: GPSMapDatum
  type_name: ASCII
- id: 0x0013
  name: GPSDestLatitudeRef
  type_name: ASCII
- id: 0x0014
  name: GPSDestLatitude
  type_name: RATIONAL
- id: 0x0015
  name: GPSDestLongitudeRef
  type_name: ASCII
- id: 0x0016
  name: GPSDestLongitude
  type_name: RATIONAL
- id: 0x0017
  name: GPSDestBearingRef
  type_name: ASCII
- id: 0x0018
  name: GPSDestBearing
  type_name: RATIONAL
- id: 0x0019
  name: GPSDestDistanceRef
  type_name: ASCII
- id: 0x001a
  name: GPSDestDistance
  type_name: RATIONAL
- id: 0x001b
  name: GPSProcessingMethod
  type_name: UNDEFINED
- id: 0x001c
  name: GPSAreaInformation
  type_name: UNDEFINED
- id: 0x001d
  name: GPSDateStamp
  type_name: ASCII
- id: 0x001e
  name: GPSDifferential
  type_name: SHORT
IFD:
- id: 0x000b
  name: ProcessingSoftware
  type_name: ASCII
- id: 0x00fe
  name: NewSubfileType
  type_name: LONG
- id: 0x00ff
  name: SubfileType
  type_name: SHORT
- id: 0x0100
  name: ImageWidth
  type_names: [LONG, SHORT]
- id: 0x0101
  name: ImageLength
  type_names: [LONG, SHORT]
- id: 0x0102
  name: BitsPerSample
  type_name: SHORT
- id: 0x0103
  name: Compression
  type_name: SHORT
- id: 0x0106
  name: PhotometricInterpretation
  type_name: SHORT
- id: 0x0107
  name: Thresholding
  type_name: SHORT
- id: 0x0108
  name: CellWidth
  type_name: SHORT
- id: 0x0109
  name: CellLength
  type_name: SHORT
- id: 0x010a
  name: FillOrder
  type_name: SHORT
- id: 0x010d
  name: DocumentName
  type_name: ASCII
- id: 0x010e
  name: ImageDescription
  type_name: ASCII
- id: 0x010f
  name: Make
  type_name: ASCII
- id: 0x0110
  name: Model
  type_name: ASCII
- id: 0x0111
  name: StripOffsets
  type_names: [LONG, SHORT]
- id: 0x0112
  name: Orientation
  type_name: SHORT
- id: 0x0115
  name: SamplesPerPixel
  type_name: SHORT
- id: 0x0116
  name: RowsPerStrip
  type_names: [LONG, SHORT]
- id: 0x0117
  name: StripByteCounts
  type_names: [LONG, SHORT]
- id: 0x011a
  name: XResolution
  type_name: RATIONAL
- id: 0x011b
  name: YResolution
  type_name: RATIONAL
- id: 0x011c
  name: PlanarConfiguration
  type_name: SHORT
- id: 0x0122
  name: GrayResponseUnit
  type_name: SHORT
- id: 0x0123
  name: GrayResponseCurve
  type_name: SHORT
- id: 0x0124
  name: T4Options
  type_name: LONG
- id: 0x0125
  name: T6Options
  type_name: LONG
- id: 0x0128
  name: ResolutionUnit
  type_name: SHORT
- id: 0x0129
  name: PageNumber
  type_name: SHORT
- id: 0x012d
  name: TransferFunction
  type_name: SHORT
- id: 0x0131
  name: Software
  type_name: ASCII
- id: 0x0132
  name: DateTime
  type_name: ASCII
- id: 0x013b
  name: Artist
  type_name: ASCII
- id: 0x013c
  name: HostComputer
  type_name: ASCII
- id: 0x013d
  name: Predictor
  type_name: SHORT
- id: 0x013e
  name: WhitePoint
  type_name: RATIONAL
- id: 0x013f
  name: PrimaryChromaticities
  type_name: RATIONAL
- id: 0x0140
  name: ColorMap
  type_name: SHORT
- id: 0x0141
  name: HalftoneHints
  type_name: SHORT
- id: 0x0142
  name: TileWidth
  type_name: SHORT
- id: 0x0143
  name: TileLength
  type_name: SHORT
- id: 0x0144
  name: TileOffsets
  type_name: SHORT
- id: 0x0145
  name: TileByteCounts
  type_name: SHORT
- id: 0x014a
  name: SubIFDs
  type_name: LONG
- id: 0x014c
  name: InkSet
  type_name: SHORT
- id: 0x014d
  name: InkNames
  type_name: ASCII
- id: 0x014e
  name: NumberOfInks
  type_name: SHORT
- id: 0x0150
  name: DotRange
  type_name: BYTE
- id: 0x0151
  name: TargetPrinter
  type_name: ASCII
- id: 0x0152
  name: ExtraSamples
  type_name: SHORT
- id: 0x0153
  name: SampleFormat
  type_name: SHORT
- id: 0x0154
  name: SMinSampleValue
  type_name: SHORT
- id: 0x0155
  name: SMaxSampleValue
  type_name: SHORT
- id: 0x0156
  name: TransferRange
  type_name: SHORT
- id: 0x0157
  name: ClipPath
  type_name: BYTE
- id: 0x015a
  name: Indexed
  type_name: SHORT
- id: 0x015b
  name: JPEGTables
  type_name: UNDEFINED
- id: 0x015f
  name: OPIProxy
  type_name: SHORT
- id: 0x0200
  name: JPEGProc
  type_name: LONG
- id: 0x0201
  name: JPEGInterchangeFormat
  type_name: LONG
- id: 0x0202
  name: JPEGInterchangeFormatLength
  type_name: LONG
- id: 0x0203
  name: JPEGRestartInterval
  type_name: SHORT
- id: 0x0205
  name: JPEGLosslessPredictors
  type_name: SHORT
- id: 0x0206
  name: JPEGPointTransforms
  type_name: SHORT
- id: 0x0207
  name: JPEGQTables
  type_name: LONG
- id: 0x0208
  name: JPEGDCTables
  type_name: LONG
- id: 0x0209
  name: JPEGACTables
  type_name: LONG
- id: 0x0211
  name: YCbCrCoefficients
  type_name: RATIONAL
- id: 0x0212
  name: YCbCrSubSampling
  type_name: SHORT
- id: 0x0213
  name: YCbCrPositioning
  type_name: SHORT
- id: 0x0214
  name: ReferenceBlackWhite
  type_name: RATIONAL
- id: 0x02bc
  name: XMLPacket
  type_name: BYTE
- id: 0x4746
  name: Rating
  type_name: SHORT
- id: 0x4749
  name: RatingPercent
  type_name: SHORT
- id: 0x800d
  name: ImageID
  type_name: ASCII
- id: 0x828d
  name: CFARepeatPatternDim
  type_name: SHORT
- id: 0x828e
  name: CFAPattern
  type_name: BYTE
- id: 0x828f
  name: BatteryLevel
  type_name: RATIONAL
- id: 0x8298
  name: Copyright
  type_name: ASCII
- id: 0x829a
  name: ExposureTime
# NOTE(dustin): SRATIONAL isn't mentioned in the standard, but we have seen it in real data.
  type_names: [RATIONAL, SRATIONAL]
- id: 0x829d
  name: FNumber
# NOTE(dustin): SRATIONAL isn't mentioned in the standard, but we have seen it in real data.
  type_names: [RATIONAL, SRATIONAL]
- id: 0x83bb
  name: IPTCNAA
  type_name: LONG
- id: 0x8649
  name: ImageResources
  type_name: BYTE
- id: 0x8769
  name: ExifTag
  type_name: LONG
- id: 0x8773
  name: InterColorProfile
  type_name: UNDEFINED
- id: 0x8822
  name: ExposureProgram
  type_name: SHORT
- id: 0x8824
  name: SpectralSensitivity
  type_name: ASCII
- id: 0x8825
  name: GPSTag
  type_name: LONG
- id: 0x8827
  name: ISOSpeedRatings
  type_name: SHORT
- id: 0x8828
  name: OECF
  type_name: UNDEFINED
- id: 0x8829
  name: Interlace
  type_name: SHORT
- id: 0x882b
  name: SelfTimerMode
  type_name: SHORT
- id: 0x9003
  name: DateTimeOriginal
  type_name: ASCII
- id: 0x9102
  name: CompressedBitsPerPixel
  type_name: RATIONAL
- id: 0x9201
  name: ShutterSpeedValue
  type_name: SRATIONAL
- id: 0x9202
  name: ApertureValue
  type_name: RATIONAL
- id: 0x9203
  name: BrightnessValue
  type_name: SRATIONAL
- id: 0x9204
  name: ExposureBiasValue
  type_name: SRATIONAL
- id: 0x9205
  name: MaxApertureValue
  type_name: RATIONAL
- id: 0x9206
  name: SubjectDistance
  type_name: SRATIONAL
- id: 0x9207
  name: MeteringMode
  type_name: SHORT
- id: 0x9208
  name: LightSource
  type_name: SHORT
- id: 0x9209
  name: Flash
  type_name: SHORT
- id: 0x920a
  name: FocalLength
  type_name: RATIONAL
- id: 0x920b
  name: FlashEnergy
  type_name: RATIONAL
- id: 0x920c
  name: SpatialFrequencyResponse
  type_name: UNDEFINED
- id: 0x920d
  name: Noise
  type_name: UNDEFINED
- id: 0x920e
  name: FocalPlaneXResolution
  type_name: RATIONAL
- id: 0x920f
  name: FocalPlaneYResolution
  type_name: RATIONAL
- id: 0x9210
  name: FocalPlaneResolutionUnit
  type_name: SHORT
- id: 0x9211
  name: ImageNumber
  type_name: LONG
- id: 0x9212
  name: SecurityClassification
  type_name: ASCII
- id: 0x9213
  name: ImageHistory
  type_name: ASCII
- id: 0x9214
  name: SubjectLocation
  type_name: SHORT
- id: 0x9215
  name: ExposureIndex
  type_name: RATIONAL
- id: 0x9216
  name: TIFFEPStandardID
  type_name: BYTE
- id: 0x9217
  name: SensingMethod
  type_name: SHORT
- id: 0x9c9b
  name: XPTitle
  type_name: BYTE
- id: 0x9c9c
  name: XPComment
  type_name: BYTE
- id: 0x9c9d
  name: XPAuthor
  type_name: BYTE
- id: 0x9c9e
  name: XPKeywords
  type_name: BYTE
- id: 0x9c9f
  name: XPSubject
  type_name: BYTE
- id: 0xc4a5
  name: PrintImageMatching
  type_name: UNDEFINED
- id: 0xc612
  name: DNGVersion
  type_name: BYTE
- id: 0xc613
  name: DNGBackwardVersion
  type_name: BYTE
- id: 0xc614
  name: UniqueCameraModel
  type_name: ASCII
- id: 0xc615
  name: LocalizedCameraModel
  type_name: BYTE
- id: 0xc616
  name: CFAPlaneColor
  type_name: BYTE
- id: 0xc617
  name: CFALayout
  type_name: SHORT
- id: 0xc618
  name: LinearizationTable
  type_name: SHORT
- id: 0xc619
  name: BlackLevelRepeatDim
  type_name: SHORT
- id: 0xc61a
  name: BlackLevel
  type_name: RATIONAL
- id: 0xc61b
  name: BlackLevelDeltaH
  type_name: SRATIONAL
- id: 0xc61c
  name: BlackLevelDeltaV
  type_name: SRATIONAL
- id: 0xc61d
  name: WhiteLevel
  type_name: SHORT
- id: 0xc61e
  name: DefaultScale
  type_name: RATIONAL
- id: 0xc61f
  name: DefaultCropOrigin
  type_name: SHORT
- id: 0xc620
  name: DefaultCropSize
  type_name: SHORT
- id: 0xc621
  name: ColorMatrix1
  type_name: SRATIONAL
- id: 0xc622
  name: ColorMatrix2
  type_name: SRATIONAL
- id: 0xc623
  name: CameraCalibration1
  type_name: SRATIONAL
- id: 0xc624
  name: CameraCalibration2
  type_name: SRATIONAL
- id: 0xc625
  name: ReductionMatrix1
  type_name: SRATIONAL
- id: 0xc626
  name: ReductionMatrix2
  type_name: SRATIONAL
- id: 0xc627
  name: AnalogBalance
  type_name: RATIONAL
- id: 0xc628
  name: AsShotNeutral
  type_name: SHORT
- id: 0xc629
  name: AsShotWhiteXY
  type_name: RATIONAL
- id: 0xc62a
  name: BaselineExposure
  type_name: SRATIONAL
- id: 0xc62b
  name: BaselineNoise
  type_name: RATIONAL
- id: 0xc62c
  name: BaselineSharpness
  type_name: RATIONAL
- id: 0xc62d
  name: BayerGreenSplit
  type_name: LONG
- id: 0xc62e
  name: LinearResponseLimit
  type_name: RATIONAL
- id: 0xc62f
  name: CameraSerialNumber
  type_name: ASCII
- id: 0xc630
  name: LensInfo
  type_name: RATIONAL
- id: 0xc631
  name: ChromaBlurRadius
  type_name: RATIONAL
- id: 0xc632
  name: AntiAliasStrength
  type_name: RATIONAL
- id: 0xc633
  name: ShadowScale
  type_name: SRATIONAL
- id: 0xc634
  name: DNGPrivateData
  type_name: BYTE
- id: 0xc635
  name: MakerNoteSafety
  type_name: SHORT
- id: 0xc65a
  name: CalibrationIlluminant1
  type_name: SHORT
- id: 0xc65b
  name: CalibrationIlluminant2
  type_name: SHORT
- id: 0xc65c
  name: BestQualityScale
  type_name: RATIONAL
- id: 0xc65d
  name: RawDataUniqueID
  type_name: BYTE
- id: 0xc68b
  name: OriginalRawFileName
  type_name: BYTE
- id: 0xc68c
  name: OriginalRawFileData
  type_name: UNDEFINED
- id: 0xc68d
  name: ActiveArea
  type_name: SHORT
- id: 0xc68e
  name: MaskedAreas
  type_name: SHORT
- id: 0xc68f
  name: AsShotICCProfile
  type_name: UNDEFINED
- id: 0xc690
  name: AsShotPreProfileMatrix
  type_name: SRATIONAL
- id: 0xc691
  name: CurrentICCProfile
  type_name: UNDEFINED
- id: 0xc692
  name: CurrentPreProfileMatrix
  type_name: SRATIONAL
- id: 0xc6bf
  name: ColorimetricReference
  type_name: SHORT
- id: 0xc6f3
  name: CameraCalibrationSignature
  type_name: BYTE
- id: 0xc6f4
  name: ProfileCalibrationSignature
  type_name: BYTE
- id: 0xc6f6
  name: AsShotProfileName
  type_name: BYTE
- id: 0xc6f7
  name: NoiseReductionApplied
  type_name: RATIONAL
- id: 0xc6f8
  name: ProfileName
  type_name: BYTE
- id: 0xc6f9
  name: ProfileHueSatMapDims
  type_name: LONG
- id: 0xc6fd
  name: ProfileEmbedPolicy
  type_name: LONG
- id: 0xc6fe
  name: ProfileCopyright
  type_name: BYTE
- id: 0xc714
  name: ForwardMatrix1
  type_name: SRATIONAL
- id: 0xc715
  name: ForwardMatrix2
  type_name: SRATIONAL
- id: 0xc716
  name: PreviewApplicationName
  type_name: BYTE
- id: 0xc717
  name: PreviewApplicationVersion
  type_name: BYTE
- id: 0xc718
  name: PreviewSettingsName
  type_name: BYTE
- id: 0xc719
  name: PreviewSettingsDigest
  type_name: BYTE
- id: 0xc71a
  name: PreviewColorSpace
  type_name: LONG
- id: 0xc71b
  name: PreviewDateTime
  type_name: ASCII
- id: 0xc71c
  name: RawImageDigest
  type_name: UNDEFINED
- id: 0xc71d
  name: OriginalRawFileDigest
  type_name: UNDEFINED
- id: 0xc71e
  name: SubTileBlockSize
  type_name: LONG
- id: 0xc71f
  name: RowInterleaveFactor
  type_name: LONG
- id: 0xc725
  name: ProfileLookTableDims
  type_name: LONG
- id: 0xc740
  name: OpcodeList1
  type_name: UNDEFINED
- id: 0xc741
  name: OpcodeList2
  type_name: UNDEFINED
- id: 0xc74e
  name: OpcodeList3
  type_name: UNDEFINED
# This tag may be used to specify the size of raster pixel spacing in the
# model space units, when the raster space can be embedded in the model space
# coordinate system without rotation, and consists of the following 3 values:
#    ModelPixelScaleTag = (ScaleX, ScaleY, ScaleZ)
# where ScaleX and ScaleY give the horizontal and vertical spacing of raster
# pixels. The ScaleZ is primarily used to map the pixel value of a digital
# elevation model into the correct Z-scale, and so for most other purposes
# this value should be zero (since most model spaces are 2-D, with Z=0).
# Source: http://geotiff.maptools.org/spec/geotiff2.6.html#2.6.1
- id: 0x830e
  name: ModelPixelScaleTag
  type_name: DOUBLE
# This tag stores raster->model tiepoint pairs in the order
#	ModelTiepointTag = (...,I,J,K, X,Y,Z...),
# where (I,J,K) is the point at location (I,J) in raster space with
# pixel-value K, and (X,Y,Z) is a vector in model space. In most cases the
# model space is only two-dimensional, in which case both K and Z should be
# set to zero; this third dimension is provided in anticipation of future
# support for 3D digital elevation models and vertical coordinate systems.
# Source: http://geotiff.maptools.org/spec/geotiff2.6.html#2.6.1
- id: 0x8482
  name: ModelTiepointTag
  type_name: DOUBLE
# This tag may be used to specify the transformation matrix between the
# raster space (and its dependent pixel-value space) and the (possibly 3D)
# model space.
# Source: http://geotiff.maptools.org/spec/geotiff2.6.html#2.6.1
- id: 0x85d8
  name: ModelTransformationTag
  type_name: DOUBLE
IFD/Exif/Iop:
- id: 0x0001
  name: InteroperabilityIndex
  type_name: ASCII
- id: 0x0002
  name: InteroperabilityVersion
  type_name: UNDEFINED
- id: 0x1000
  name: RelatedImageFileFormat
  type_name: ASCII
- id: 0x1001
  name: RelatedImageWidth
  type_name: LONG
- id: 0x1002
  name: RelatedImageLength
  type_name: LONG
`
)
