package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readResourceReviewContract(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{"..", "..", ".."}, parts...)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestReviewerDefinesMapperAndYamlResourceScope(t *testing.T) {
	text := readResourceReviewContract(t, "agents", "reviewer.md")
	for _, want := range []string{
		"MapperXml",
		"YamlConfig",
		"resourceRelations",
		"*Mapper.xml",
		"src/main/resources/**/*.yml",
		"FULL",
		"TARGETED",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("reviewer resource contract missing %q", want)
		}
	}
}

func TestAnalyzeChangeRequiresResourceCoverageAndEvidenceRelations(t *testing.T) {
	text := readResourceReviewContract(t, "skills", "analyze-change", "SKILL.md")
	for _, want := range []string{
		"mapperIncludes",
		"configIncludes",
		"MapperXml",
		"YamlConfig",
		"resourceRelations",
		"MAPPER_STATEMENT",
		"CONFIG_REFERENCE",
		"无法证明关联时不得加入 TARGETED scopedFiles",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("analyze-change resource rule missing %q", want)
		}
	}
}

func TestReviewCodeAllowsOnlyHighValueMapperFindings(t *testing.T) {
	text := readResourceReviewContract(t, "skills", "review-code", "SKILL.md")
	for _, want := range []string{
		"UPDATE / DELETE",
		"WHERE",
		"租户",
		"动态 SQL",
		"statement id",
		"参数",
		"resultMap/resultType",
		"无边界批量更新/删除",
		"不得因为 XML 格式、缩进、命名风格产生 Finding",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("review-code Mapper rule missing %q", want)
		}
	}
}

func TestReviewCodeAllowsOnlyChangedHighValueYamlFindings(t *testing.T) {
	text := readResourceReviewContract(t, "skills", "review-code", "SKILL.md")
	for _, want := range []string{
		"datasource",
		"timeout",
		"Redis/MQ/RPC",
		"日志级别",
		"feature switch",
		"敏感信息",
		"@Value",
		"@ConfigurationProperties",
		"不得对未变化的配置做泛化审查",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("review-code YML rule missing %q", want)
		}
	}
}
