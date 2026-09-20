package changeset

import "testing"

func Test162MultiModuleChangeSetPreservesRepoRelativePathsAcrossGitSources(t *testing.T) {
	repo := new153GitRepo(t)
	write153(t, repo, "src/main/java/acme/Root.java", "class Root { int v = 1; }\n")
	write153(t, repo, "module-a/src/main/java/com/acme/UserService.java", "class UserService { int v = 1; }\n")
	write153(t, repo, "module-b/src/main/java/com/acme/UserService.java", "class UserService { int v = 1; }\n")
	write153(t, repo, "module-a/src/test/java/com/acme/UserServiceTest.java", "class UserServiceTest { int v = 1; }\n")
	write153(t, repo, "module-dao/src/main/resources/mapper/UserMapper.xml", "<mapper><select id=\"a\">1</select></mapper>\n")
	write153(t, repo, "module-service/src/main/resources/application.yml", "feature: false\n")
	git153(t, repo, "add", ".")
	git153(t, repo, "commit", "-m", "base")

	write153(t, repo, "src/main/java/acme/Root.java", "class Root { int v = 2; }\n")
	git153(t, repo, "add", "src/main/java/acme/Root.java")
	git153(t, repo, "commit", "-m", "root committed")

	write153(t, repo, "module-a/src/main/java/com/acme/UserService.java", "class UserService { int v = 2; }\n")
	git153(t, repo, "add", "module-a/src/main/java/com/acme/UserService.java")
	git153(t, repo, "commit", "-m", "module committed")

	write153(t, repo, "module-b/src/main/java/com/acme/UserService.java", "class UserService { int v = 2; }\n")
	git153(t, repo, "add", "module-b/src/main/java/com/acme/UserService.java")
	write153(t, repo, "module-dao/src/main/resources/mapper/UserMapper.xml", "<mapper><select id=\"a\">2</select></mapper>\n")
	write153(t, repo, "module-service/src/main/resources/application.yml", "feature: true\n")
	write153(t, repo, "module-a/src/test/java/com/acme/UserServiceTest.java", "class UserServiceTest { int v = 2; }\n")

	snap, err := Compute(repo, "HEAD~2", true)
	if err != nil { t.Fatal(err) }

	assert153Source(t, snap, "src/main/java/acme/Root.java", SourceCommitted)
	assert153Source(t, snap, "module-a/src/main/java/com/acme/UserService.java", SourceCommitted)
	assert153Source(t, snap, "module-b/src/main/java/com/acme/UserService.java", SourceStaged)
	assert153Source(t, snap, "module-dao/src/main/resources/mapper/UserMapper.xml", SourceUnstaged)
	assert153Source(t, snap, "module-service/src/main/resources/application.yml", SourceUnstaged)
	assert153Source(t, snap, "module-a/src/test/java/com/acme/UserServiceTest.java", SourceUnstaged)

	write153(t, repo, "module-new/src/main/java/com/acme/NewService.java", "class NewService {}\n")
	write153(t, repo, "module-new/src/test/java/com/acme/NewServiceTest.java", "class NewServiceTest {}\n")
	write153(t, repo, "module-new/src/main/resources/mapper/NewMapper.xml", "<mapper/>\n")
	write153(t, repo, "module-new/src/main/resources/application.yml", "enabled: true\n")

	snap, err = Compute(repo, "HEAD~2", true)
	if err != nil { t.Fatal(err) }
	for _, p := range []string{
		"module-new/src/main/java/com/acme/NewService.java",
		"module-new/src/test/java/com/acme/NewServiceTest.java",
		"module-new/src/main/resources/mapper/NewMapper.xml",
		"module-new/src/main/resources/application.yml",
	} {
		assert153Source(t, snap, p, SourceUntracked)
	}

	if file153(t, snap, "module-a/src/main/java/com/acme/UserService.java").Path == file153(t, snap, "module-b/src/main/java/com/acme/UserService.java").Path {
		t.Fatal("module prefixes must remain part of repo-relative identity")
	}
}
