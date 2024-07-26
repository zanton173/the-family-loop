package vars

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	firebase "firebase.google.com/go"
	"google.golang.org/api/option"
)

var Replacer *strings.Replacer = strings.NewReplacer("'", "\\'", "\"", "\\\"")

var NyLoc, NyLocErr = time.LoadLocation("America/New_York")
var FbOpts = []option.ClientOption{option.WithCredentialsFile("the-family-loop-fb0d9-firebase-adminsdk-k6sxl-14c7d4c4f7.json")}
var App, AppErr = firebase.NewApp(context.TODO(), nil, FbOpts...)
var Fb_message_client, FbInitErr = App.Messaging(context.TODO())

func DbConn() *sql.DB {
	dbpass := os.Getenv("DB_PASS")

	connStr := fmt.Sprintf("postgresql://tfldbrole:%s@localhost/tfl?sslmode=disable", dbpass)
	db, err := sql.Open("postgres", connStr)

	if err != nil {
		log.Fatal(err)
	}
	//defer db.Close()
	return db
}
