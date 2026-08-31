package projectpath

import (
	"path"
	"strings"
)

type Kind string

const (
	Other      Kind = "OTHER"
	MainJava   Kind = "MAIN_JAVA"
	TestJava   Kind = "TEST_JAVA"
	MapperXML  Kind = "MAPPER_XML"
	YAMLConfig Kind = "YAML_CONFIG"
)

func Normalize(value string) (string, bool) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || path.IsAbs(value) {
		return "", false
	}
	clean := path.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}

func Classify(value string) Kind {
	p, ok := Normalize(value)
	if !ok {
		return Other
	}
	switch {
	case underSourceRoot(p, "src/main/java/") && strings.HasSuffix(p, ".java"):
		return MainJava
	case underSourceRoot(p, "src/test/java/") && strings.HasSuffix(p, ".java"):
		return TestJava
	case underSourceRoot(p, "src/main/resources/") && strings.HasSuffix(path.Base(p), "Mapper.xml"):
		return MapperXML
	case underSourceRoot(p, "src/main/resources/") && strings.HasSuffix(p, ".yml"):
		return YAMLConfig
	default:
		return Other
	}
}

func IsMainJava(value string) bool  { return Classify(value) == MainJava }
func IsTestJava(value string) bool  { return Classify(value) == TestJava }
func IsMapperXML(value string) bool { return Classify(value) == MapperXML }
func IsYAMLConfig(value string) bool { return Classify(value) == YAMLConfig }
func IsReviewPath(value string) bool { return Classify(value) != Other }

func underSourceRoot(p, root string) bool {
	return strings.HasPrefix(p, root) || strings.Contains(p, "/"+root)
}
