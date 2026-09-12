package reviewcontext

import (
	"context"
	"testing"
)

func Test170MyBatisKnownMapperDTDIsInert(t *testing.T) {
	root, source := newMyBatisFixture170(t, map[string]string{
		"src/main/java/demo/OrderMapper.java": `package demo;
public interface OrderMapper {
    int updateStatus(Long id);
}
`,
		"src/main/resources/OrderMapper.xml": `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE mapper PUBLIC "-//mybatis.org//DTD Mapper 3.0//EN" "http://mybatis.org/dtd/mybatis-3-mapper.dtd">
<mapper namespace="demo.OrderMapper">
  <update id="updateStatus">UPDATE orders SET status = 'DONE' WHERE id = #{id}</update>
</mapper>
`,
	}, "demo.OrderMapper", "updateStatus")

	relations, issues, err := ResolveMapper170(context.Background(), root, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("issues=%+v", issues)
	}
	statement := relationByKind170(t, relations, "MYBATIS_STATEMENT")
	if statement.Resolution != "EXACT" {
		t.Fatalf("standard MyBatis DTD must be treated as inert declaration, statement=%+v", statement)
	}
}
