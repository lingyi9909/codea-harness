package main

import (
	"os"
	"testing"
)

func setup151FreshGitFixture(t *testing.T) {
	t.Helper()
	git151(t, "init", "-b", "develop")
	git151(t, "config", "user.email", "codea@example.invalid")
	git151(t, "config", "user.name", "Codea Test")
	git151(t, "add", ".code-harness/contracts/change-analysis.schema.json")
	git151(t, "commit", "-m", "baseline")
	git151(t, "checkout", "-b", "feature/new-approval")

	writeFile(t, "src/main/java/com/example/approval/NewController.java", `package com.example.approval;
@RestController
public class NewController {
    private final ApprovalService approvalService;
    public NewController(ApprovalService approvalService) { this.approvalService = approvalService; }
    @PostMapping("/approve")
    public void approve() { approvalService.approve(); }
}
`)
	writeFile(t, "src/main/java/com/example/approval/ApprovalService.java", `package com.example.approval;
public interface ApprovalService { void approve(); }
`)
	writeFile(t, "src/main/java/com/example/approval/ApprovalServiceImpl.java", `package com.example.approval;
@Service
public class ApprovalServiceImpl implements ApprovalService {
    private final ApprovalMapper mapper;
    public ApprovalServiceImpl(ApprovalMapper mapper) { this.mapper = mapper; }
    public void approve() { mapper.updateStatus(); }
}
`)
	writeFile(t, "src/main/java/com/example/approval/ApprovalMapper.java", `package com.example.approval;
@Mapper
public interface ApprovalMapper { void updateStatus(); }
`)
	writeFile(t, "src/main/resources/mapper/ApprovalMapper.xml", `<mapper namespace="com.example.approval.ApprovalMapper"><update id="updateStatus">update approval set status='DONE'</update></mapper>`)

	git151(t, "add",
		"src/main/java/com/example/approval/NewController.java",
		"src/main/java/com/example/approval/ApprovalService.java",
		"src/main/java/com/example/approval/ApprovalServiceImpl.java",
		"src/main/java/com/example/approval/ApprovalMapper.java",
	)
	f, err := os.OpenFile("src/main/java/com/example/approval/ApprovalServiceImpl.java", os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("// unstaged follow-up\n"); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
