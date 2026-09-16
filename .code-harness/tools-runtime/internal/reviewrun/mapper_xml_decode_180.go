package reviewrun

import (
	"encoding/xml"
	"io"
)

// UnmarshalXML decodes the MyBatis mapper contract used by Task 2. Both the
// mapper namespace and statement id are XML attributes, not child elements.
func (m *mapperXML180) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	m.Namespace = ""
	m.Statements = nil
	for _, attr := range start.Attr {
		if attr.Name.Local == "namespace" {
			m.Namespace = attr.Value
			break
		}
	}
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			id := ""
			for _, attr := range v.Attr {
				if attr.Name.Local == "id" {
					id = attr.Value
					break
				}
			}
			if id != "" {
				var stmt struct {
					XMLName xml.Name
					ID      string `xml:"id"`
				}
				stmt.XMLName = v.Name
				stmt.ID = id
				m.Statements = append(m.Statements, stmt)
			}
			if err := d.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			if v.Name == start.Name {
				return nil
			}
		}
	}
}
