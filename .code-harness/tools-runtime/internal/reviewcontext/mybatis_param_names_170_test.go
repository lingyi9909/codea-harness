package reviewcontext

import (
	"context"
	"testing"
)

func Test170MyBatisBusinessParamPrefixStillRequiresExplicitContract(t *testing.T) {
	root, source := newMyBatisFixture170(t, map[string]string{
		"src/main/java/demo/OrderMapper.java": `package demo;
import org.apache.ibatis.annotations.Param;
public interface OrderMapper {
    int updateStatus(@Param("id") Long id);
}
`,
		"src/main/resources/OrderMapper.xml": `<mapper namespace="demo.OrderMapper">
  <update id="updateStatus">UPDATE orders SET status = 'DONE' WHERE id = #{parameterCode} OR id = #{argument}</update>
</mapper>
`,
	}, "demo.OrderMapper", "updateStatus")

	_, issues, err := ResolveMapper170(context.Background(), root, source)
	if err != nil {
		t.Fatal(err)
	}
	if !hasIssue170(issues, "MYBATIS_PARAM_CONTRACT_MISMATCH") {
		t.Fatalf("business names beginning with param/arg must not be treated as generated aliases: %+v", issues)
	}
}
