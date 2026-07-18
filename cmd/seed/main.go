// Command seed generates a re-runnable SQL dump that bulk-inserts fake users
// into the "user" table, for import via TablePlus (Import > From SQL Dump).
//
//	go run ./cmd/seed -n 50000 -o seed_users.sql
//
// Every account shares one bcrypt-hashed password (default "12345678") so any
// seeded account is usable for login during testing.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"yupao-go/internal/shared/usertype"

	"golang.org/x/crypto/bcrypt"
)

const (
	roleDefault    = 0
	roleAdmin      = 1
	adminEveryN    = 1000
	secondsPerYear = 365 * 24 * 60 * 60
)

var (
	surnames   = []rune("赵钱孙李周吴郑王冯陈褚卫蒋沈韩杨朱秦尤许何吕施张孔曹严华金魏陶姜戚谢邹喻")
	givenChars = []rune("伟芳娜秀英敏静丽强磊军洋勇艳杰娟涛明超霞平刚桂华建文志鹏飞燕玲凤云梅雪龙")
	genders    = []usertype.Gender{usertype.GenderMale, usertype.GenderFemale}
	tagPool    = []string{
		"Go", "Java", "Python", "JavaScript", "TypeScript", "React", "Vue",
		"Angular", "Node", "Rust", "C++", "C#", "Ruby", "PHP", "Swift",
		"Kotlin", "Gin", "Spring", "Django", "Flask", "MySQL", "PostgreSQL",
		"Redis", "Docker", "Kubernetes", "算法", "后端", "前端", "全栈",
		"篮球", "足球", "游泳", "音乐", "电影", "读书", "旅行", "健身",
		"摄影", "游戏", "male", "female", "单身", "已婚",
	}
)

func main() {
	var (
		n        = flag.Int("n", 50000, "number of users to generate")
		out      = flag.String("o", "seed_users.sql", "output SQL file path")
		password = flag.String("password", "12345678", "plaintext password (bcrypt-hashed once, shared by all rows)")
		batch    = flag.Int("batch", 1000, "rows per INSERT statement")
		start    = flag.Int("start", 1, "starting index for account/planet numbering (bump this on re-runs to avoid overlap)")
		seed     = flag.Int64("seed", 1, "random seed for reproducible output")
	)
	flag.Parse()

	if *n <= 0 {
		log.Fatal("n must be > 0")
	}
	if *batch <= 0 {
		log.Fatal("batch must be > 0")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("bcrypt: %v", err)
	}
	pwd := sqlStr(string(hashed))

	rng := rand.New(rand.NewSource(*seed))

	f, err := os.Create(*out)
	if err != nil {
		log.Fatalf("create file: %v", err)
	}
	defer f.Close()

	w := bufio.NewWriterSize(f, 1<<20)

	const cols = `"user_account", "username", "user_password", "planet_code", "gender", "phone", "email", "user_role", "tags", "create_time", "update_time"`

	fmt.Fprintf(w, "-- Auto-generated seed for the \"user\" table: %d rows.\n", *n)
	fmt.Fprintf(w, "-- Plaintext password for every account: %q (bcrypt-hashed).\n", *password)
	fmt.Fprintf(w, "-- Safe to re-run: ON CONFLICT (user_account) DO NOTHING.\n\n")

	now := time.Now()
	for i := 0; i < *n; i++ {
		switch {
		case i%*batch == 0:
			if i > 0 {
				w.WriteString("\nON CONFLICT (\"user_account\") DO NOTHING;\n\n")
			}
			fmt.Fprintf(w, "INSERT INTO \"user\" (%s) VALUES\n", cols)
		default:
			w.WriteString(",\n")
		}

		idx := *start + i
		account := fmt.Sprintf("user_%06d", idx)
		planet := fmt.Sprintf("P%08d", idx)
		gender := genders[rng.Intn(len(genders))]
		phone := fmt.Sprintf("1%d%09d", 3+rng.Intn(7), rng.Intn(1_000_000_000))
		role := roleDefault
		if rng.Intn(adminEveryN) == 0 {
			role = roleAdmin
		}
		ct := now.Add(-time.Duration(rng.Intn(secondsPerYear)) * time.Second)
		ts := sqlStr(ct.Format("2006-01-02 15:04:05-07:00"))

		fmt.Fprintf(w, "(%s, %s, %s, %s, %d, %s, %s, %d, %s, %s, %s)",
			sqlStr(account), sqlStr(randName(rng)), pwd, sqlStr(planet),
			gender, sqlStr(phone), sqlStr(account+"@example.com"), role, sqlStr(randTags(rng)), ts, ts)
	}
	w.WriteString("\nON CONFLICT (\"user_account\") DO NOTHING;\n")

	if err := w.Flush(); err != nil {
		log.Fatalf("flush: %v", err)
	}
	if err := f.Close(); err != nil {
		log.Fatalf("close: %v", err)
	}
	fmt.Printf("wrote %d rows to %s\n", *n, *out)
}

func randName(rng *rand.Rand) string {
	var b strings.Builder
	b.WriteRune(surnames[rng.Intn(len(surnames))])
	for j, k := 0, 1+rng.Intn(2); j < k; j++ {
		b.WriteRune(givenChars[rng.Intn(len(givenChars))])
	}
	return b.String()
}

func randTags(rng *rand.Rand) string {
	k := 1 + rng.Intn(4)
	perm := rng.Perm(len(tagPool))
	picked := make([]string, 0, k)
	for j := 0; j < k; j++ {
		picked = append(picked, tagPool[perm[j]])
	}
	b, _ := json.Marshal(picked)
	return string(b)
}

// sqlStr renders s as a single-quoted SQL literal, doubling embedded quotes to
// prevent broken statements / injection from generated values.
func sqlStr(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
