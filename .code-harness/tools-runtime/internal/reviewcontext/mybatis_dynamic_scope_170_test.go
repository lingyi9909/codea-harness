package reviewcontext

import (
	"context"
	"testing"
)

func Test170MyBatisDynamicLocalsAreValidInsideTheirScope(t *testing.T) {
	root, source := newMyBatisFixture170(t, map[string]string{
		"src/main/java/demo/OrderMapper.java": `package demo;
import java.util.List;
import org.apache.ibatis.annotations.Param;
public interface OrderMapper {
    int search(@Param("ids") List<Long> ids, @Param("title") String title);
}
`,
		"src/main/resources/OrderMapper.xml": `<mapper namespace="demo.OrderMapper">
  <select id="search" resultType="long">
    <bind name="pattern" value="'%' + title + '%'"/>
    SELECT id FROM orders WHERE title LIKE #{pattern} AND id IN
    <foreach collection="ids" item="id" index="idx" open="(" separator="," close=")">
      #{id}
    </foreach>
  </select>
</mapper>
`,
	}, "demo.OrderMapper", "search")

	_, issues, err := ResolveMapper170(context.Background(), root, source)
	if err != nil {
		t.Fatal(err)
	}
	if hasIssue170(issues, "MYBATIS_PARAM_CONTRACT_MISMATCH") {
		t.Fatalf("foreach item/index and bind names are legal dynamic locals, issues=%+v", issues)
	}
}

func Test170MyBatisForEachLocalsDoNotLeakOutsideScope(t *testing.T) {
	root, source := newMyBatisFixture170(t, map[string]string{
		"src/main/java/demo/OrderMapper.java": `package demo;
import java.util.List;
import org.apache.ibatis.annotations.Param;
public interface OrderMapper {
    int search(@Param("ids") List<Long> ids);
}
`,
		"src/main/resources/OrderMapper.xml": `<mapper namespace="demo.OrderMapper">
  <select id="search" resultType="long">
    SELECT id FROM orders WHERE id IN
    <foreach collection="ids" item="id" open="(" separator="," close=")">
      #{id}
    </foreach>
    OR fallback_id = #{id}
  </select>
</mapper>
`,
	}, "demo.OrderMapper", "search")

	_, issues, err := ResolveMapper170(context.Background(), root, source)
	if err != nil {
		t.Fatal(err)
	}
	if !hasIssue170(issues, "MYBATIS_PARAM_CONTRACT_MISMATCH") {
		t.Fatalf("foreach item must remain local to the foreach body, issues=%+v", issues)
	}
}
