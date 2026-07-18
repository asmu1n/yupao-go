package test

import (
	"context"
	"fmt"
	"testing"
	"yupao-go/ent/user"

	"golang.org/x/crypto/bcrypt"
)

func TestDBConnection(t *testing.T) {
	ctx := context.Background()

	if err := testDB.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Log("migration ok")
}

func TestQueryUserCount(t *testing.T) {
	ctx := context.Background()

	count, err := Client().User.Query().Count(ctx)
	if err != nil {
		t.Fatalf("query users count: %v", err)
	}
	t.Logf("found %d users", count)
}

func TestSeedUsers(t *testing.T) {
	ctx := context.Background()
	hashed, _ := bcrypt.GenerateFromPassword([]byte("12345678"), bcrypt.DefaultCost)
	pwd := string(hashed)

	seeds := []struct {
		account    string
		username   string
		planetCode string
		role       int
		tags       string
	}{
		{"admin", "管理员", "00001", 1, `["Go","Java","管理"]`},
		{"zhangsan", "张三", "00002", 0, `["Java","Python","篮球"]`},
		{"lisi", "李四", "00003", 0, `["Go","React","游泳"]`},
		{"wangwu", "王五", "00004", 0, `["Java","Vue","足球"]`},
		{"zhaoliu", "赵六", "00005", 0, `["Python","Django","音乐"]`},
		{"sunqi", "孙七", "00006", 0, `["Go","Gin","篮球"]`},
		{"zhouba", "周八", "00007", 0, `["React","TypeScript","游泳"]`},
		{"wujiu", "吴九", "00008", 0, `["Java","Spring","足球"]`},
		{"zhengshi", "郑十", "00009", 0, `["Go","Python","音乐"]`},
		{"liuyi", "刘一", "00010", 0, `["Vue","React","篮球"]`},
	}

	for _, s := range seeds {
		u, err := Client().User.Create().
			SetUserAccount(s.account).
			SetUsername(s.username).
			SetUserPassword(pwd).
			SetPlanetCode(s.planetCode).
			SetUserRole(s.role).
			SetTags(s.tags).
			Save(ctx)
		if err != nil {
			t.Logf("skip %s: %v", s.account, err)
			continue
		}
		t.Logf("created: id=%d account=%s", u.ID, u.UserAccount)
	}

	count, _ := Client().User.Query().Count(ctx)
	fmt.Printf("total users: %d\n", count)
}

func TestUpdateUser(t *testing.T) {
	ctx := context.Background()
	hashed, _ := bcrypt.GenerateFromPassword([]byte("12345678"), bcrypt.DefaultCost)
	pwd := string(hashed)

	_, err := Client().User.Update().Where(user.UserAccountEQ("admin")).SetUserPassword(pwd).Save(ctx)
	if err != nil {
		t.Logf("failed update user: %v", err)
	}
	fmt.Printf("success update pwd")
}
