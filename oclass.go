package orient

import "log"

type OClass struct {
	Name             string
	ShortName        string
	Properties       map[string]*OProperty
	DefaultClusterId int32
	ClusterIds       []int32
	SuperClass       string
	OverSize         float32
	StrictMode       bool
	AbstractClass    bool
	ClusterSelection string // OClusterSelectionStrategy in Java code - needed?
	CustomFields     map[string]string
}

// Should be passed an Document that comes from a load schema
// request to the database.
func NewOClassFromDocument(doc *Document) *OClass {
	oclass := &OClass{Properties: make(map[string]*OProperty)}

	if fld := doc.GetField("name"); fld != nil && fld.Value != nil {
		oclass.Name = fld.Value.(string)
	}
	if fld := doc.GetField("shortName"); fld != nil && fld.Value != nil {
		oclass.ShortName = fld.Value.(string)
	}

	// properties comes back as an Document
	if fld := doc.GetField("properties"); fld != nil && fld.Value != nil {
		propsDocs := convertToODocumentRefSlice(fld.Value.([]interface{}))
		for _, propDoc := range propsDocs {
			oprop := NewOPropertyFromDocument(propDoc)
			oclass.Properties[oprop.Name] = oprop
		}
	}
	if fld := doc.GetField("defaultClusterId"); fld != nil && fld.Value != nil {
		oclass.DefaultClusterId = fld.Value.(int32)
	}
	if fld := doc.GetField("clusterIds"); fld != nil && fld.Value != nil {
		oclass.ClusterIds = convertToInt32Slice(fld.Value.([]interface{}))
	}
	if fld := doc.GetField("superClass"); fld != nil && fld.Value != nil {
		oclass.SuperClass = fld.Value.(string)
	}
	if fld := doc.GetField("overSize"); fld != nil && fld.Value != nil {
		oclass.OverSize = fld.Value.(float32)
	}
	if fld := doc.GetField("strictMode"); fld != nil && fld.Value != nil {
		oclass.StrictMode = fld.Value.(bool)
	}
	if fld := doc.GetField("abstract"); fld != nil && fld.Value != nil {
		oclass.AbstractClass = fld.Value.(bool)
	}
	if fld := doc.GetField("clusterSelection"); fld != nil && fld.Value != nil {
		oclass.ClusterSelection = fld.Value.(string)
	}
	if fld := doc.GetField("customFields"); fld != nil && fld.Value != nil {
		if m, ok := fld.Value.(map[string]string); ok {
			oclass.CustomFields = m
		} else {
			log.Printf("unknown type for customFields: %T\n", fld.Value)
			oclass.CustomFields = make(map[string]string)
		}
	}

	return oclass
}

func convertToODocumentRefSlice(x []interface{}) []*Document {
	y := make([]*Document, len(x))
	for i, v := range x {
		y[i] = v.(*Document)
	}
	return y
}

func convertToInt32Slice(x []interface{}) []int32 {
	y := make([]int32, len(x))
	for i, v := range x {
		y[i] = v.(int32)
	}
	return y
}
