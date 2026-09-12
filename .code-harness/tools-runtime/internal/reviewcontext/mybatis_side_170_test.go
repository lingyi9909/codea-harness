package reviewcontext

import (
	"context"
	"testing"
)

func Test170MyBatisRelationsPreserveSuppliedSourceSide(t *testing.T) {
	root, source := newMyBatisFixture170(t, map[string]string{
		"src/main/java/demo/OrderMapper.java": `package demo;
public interface OrderMapper {
    int updateStatus(Long id);
}
`,
		"src/main/resources/OrderMapper.xml": `<mapper namespace="demo.OrderMapper">
  <sql id="byId">id = #{id}</sql>
  <update id="updateStatus">UPDATE orders SET status = 'DONE' WHERE <include refid="byId"/></update>
</mapper>
`,
	}, "demo.OrderMapper", "updateStatus")
	source.Ref.Side = "BASE"

	relations, _, err := ResolveMapper170(context.Background(), root, source)
	if err != nil {
		t.Fatal(err)
	}
	for _, relation := range relations {
		if relation.From.Side != "BASE" {
			t.Fatalf("relation did not preserve BASE source side: %+v", relation)
		}
		for _, target := range relation.Targets {
			if target.Side != "BASE" {
				t.Fatalf("target did not preserve BASE source side: %+v", relation)
			}
		}
		for _, evidence := range relation.Evidence {
			if evidence.Ref.Side != "BASE" {
				t.Fatalf("evidence did not preserve BASE source side: %+v", relation)
			}
		}
	}
}
