// Copyright 2020 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fields"
	"github.com/hexya-erp/hexya/src/tools/b64image"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
)

var fields_ImageMixin = map[string]models.FieldDefinition{
	"Image1920": fields.Binary{String: "Image", JSON: "image_1920"},
	"Image1024": fields.Binary{String: "Image", JSON: "image_1024", Compute: h.ImageMixin().Methods().ComputeImages(),
		Depends: []string{"Image1920"}, Stored: true},
	"Image512": fields.Binary{String: "Image", JSON: "image_512", Compute: h.ImageMixin().Methods().ComputeImages(),
		Depends: []string{"Image1920"}, Stored: true},
	"Image256": fields.Binary{String: "Image", JSON: "image_256", Compute: h.ImageMixin().Methods().ComputeImages(),
		Depends: []string{"Image1920"}, Stored: true},
	"Image128": fields.Binary{String: "Image", JSON: "image_128", Compute: h.ImageMixin().Methods().ComputeImages(),
		Depends: []string{"Image1920"}, Stored: true},
}

// ComputeImages computes and store resized images
func imageMixin_ComputeImages(rs m.ImageMixinSet) m.ImageMixinData {
	data := h.ImageMixin().NewData()
	data.SetImage1024(b64image.Resize(rs.Image1920(), 1024, 1024, true))
	data.SetImage512(b64image.Resize(rs.Image1920(), 512, 512, true))
	data.SetImage256(b64image.Resize(rs.Image1920(), 256, 256, true))
	data.SetImage128(b64image.Resize(rs.Image1920(), 128, 128, true))
	return data
}

func init() {
	models.NewMixinModel("ImageMixin")
	h.ImageMixin().AddFields(fields_ImageMixin)
	h.ImageMixin().NewMethod("ComputeImages", imageMixin_ComputeImages)
}
