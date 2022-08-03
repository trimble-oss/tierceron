package ttdirender

import (
	"github.com/g3n/engine/graphic"
)

type CompoundMesh struct {
	graphic.Graphic
	meshes []*graphic.Mesh
}

func NewCompoundMesh(meshes []*graphic.Mesh) *CompoundMesh {
	cm := new(CompoundMesh)
	cm.meshes = meshes
	for i := 0; i < len(meshes); i += 1 {
		m := meshes[i]
		m.Init(m.IGeometry(), m.GetMaterial(0))
	}
	return cm
}
