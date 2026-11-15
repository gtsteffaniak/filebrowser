package opensw

import (
	"fmt"
	"math"
)

// Part display-mesh reading. SolidWorks parts keep the tessellation they
// draw (Contents/DisplayLists) in the same strip arrays as old assemblies;
// this is the fallback when the exact B-rep → STEP conversion cannot be
// written (OpenSW's STEP writer rejects bodies with curves it cannot
// evaluate, e.g. SP_CURVE or TRIMMED_CURVE).

// ReadPartDisplayMesh returns the part's embedded display mesh in the page's
// 3D view format: millimetres, centred, one body, faces numbered.
func ReadPartDisplayMesh(data []byte) (*View, error) {
	c, err := OpenContainer(data)
	if err != nil {
		return nil, err
	}
	dl := c.Get("Contents/DisplayLists")
	if dl == nil {
		return nil, fmt.Errorf("%w: no Contents/DisplayLists stream", AssemblyError)
	}
	faces := blockFaces(dl, 0, len(dl))
	if len(faces) == 0 {
		return nil, fmt.Errorf("%w: no display mesh decoded from part", AssemblyError)
	}
	lo := [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
	hi := [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for _, face := range faces {
		for _, tri := range face {
			for _, p := range tri {
				for j := 0; j < 3; j++ {
					lo[j] = math.Min(lo[j], p[j])
					hi[j] = math.Max(hi[j], p[j])
				}
			}
		}
	}
	var centre [3]float64
	diag := 0.0
	for j := 0; j < 3; j++ {
		centre[j] = (lo[j] + hi[j]) / 2
		diag += (hi[j] - lo[j]) * (hi[j] - lo[j])
	}
	diag = 1000 * math.Sqrt(diag)

	v := &View{Bodies: 1, Radius: diag / 2}
	for k, face := range faces {
		for _, tri := range face {
			for _, p := range tri {
				for j := 0; j < 3; j++ {
					v.Vertices = append(v.Vertices, float32(1000*(p[j]-centre[j])))
				}
			}
		}
		v.Faces = append(v.Faces, ViewFace{
			Name:      fmt.Sprintf("Face %d", k+1),
			Body:      1,
			Triangles: len(face),
		})
	}
	return v, nil
}
