package gameshandler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	globalfunctions "tfl/functions"
	globalvars "tfl/vars"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func GetStackerzLeaderboardHandler(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.URL.Query().Get("leaderboardType") == "family" {
		output, outerr := globalvars.Db.Query("select substr(username,0,14), bonus_points, level from tfldata.stack_leaderboard order by(bonus_points+level) desc limit 20;")
		if outerr != nil {
			fmt.Println(outerr)
		}
		defer output.Close()
		iter := 1
		for output.Next() {
			var username string
			var bonus_points string
			var level string
			scnerr := output.Scan(&username, &bonus_points, &level)
			if scnerr != nil {
				fmt.Println(scnerr)
			}
			dataStr := "<div class='py-0 my-0' style='display: inline-flex;'><p class='px-2 m-0' style='position: absolute; left: 2%;'>" + fmt.Sprintf("%d", iter) + ".)&nbsp;&nbsp;</p><p class='px-2 m-0' style='text-align: center; position: absolute; left: 15%;'>" + username + "</p><p class='px-2 m-0' style='text-align: center; position: relative; left: 25%;'>" + bonus_points + "</p><p class='px-2 m-0' style='text-align: center; position: absolute; left: 75%;'>" + level + "</p></div><br/>"
			iter++
			w.Write([]byte(dataStr))
		}
	} else if r.URL.Query().Get("leaderboardType") == "global" {
		eventYearConverted, _ := strconv.Atoi(r.URL.Query().Get("eventYear"))
		eventPeriodConverted, _ := strconv.Atoi(r.URL.Query().Get("eventPeriod"))
		var startPeriodMonth int
		var endPeriodMonth int
		switch eventPeriodConverted {
		case 1:
			startPeriodMonth = 0
			endPeriodMonth = 4
		case 2:
			startPeriodMonth = 3
			endPeriodMonth = 7
		case 3:
			startPeriodMonth = 6
			endPeriodMonth = 10
		case 4:
			startPeriodMonth = 9
			endPeriodMonth = 13
		}

		out, err := globalvars.Leaderboardcoll.Aggregate(context.TODO(), bson.A{
			bson.D{{Key: "$match", Value: bson.D{{Key: "game", Value: "stackerz"}}}},
			bson.D{
				{Key: "$set",
					Value: bson.D{
						{Key: "score",
							Value: bson.D{
								{Key: "$sum",
									Value: bson.A{
										"$bonus_points",
										"$level",
									},
								},
							},
						},
					},
				},
			},
			bson.D{
				{Key: "$set",
					Value: bson.D{
						{Key: "year",
							Value: bson.D{
								{Key: "$abs",
									Value: bson.D{
										{Key: "$subtract",
											Value: bson.A{
												2020,
												bson.D{{Key: "$year", Value: "$createdOn"}},
											},
										},
									},
								},
							},
						},
						{Key: "month", Value: bson.D{{Key: "$month", Value: "$createdOn"}}},
						{Key: "day", Value: bson.D{
							{Key: "$subtract",
								Value: bson.A{
									bson.D{{Key: "$dayOfWeek", Value: "$createdOn"}},
									1,
								},
							},
						},
						},
					},
				},
			},
			bson.D{
				{Key: "$match",
					Value: bson.D{
						{Key: "year", Value: eventYearConverted},
						{Key: "month",
							Value: bson.D{
								{Key: "$gt", Value: startPeriodMonth},
								{Key: "$lt", Value: endPeriodMonth},
							},
						},
						{Key: "$and",
							Value: bson.A{
								bson.D{{Key: "day", Value: bson.D{{Key: "$lt", Value: 22}}}},
								bson.D{
									{Key: "month",
										Value: bson.D{
											{Key: "$ne",
												Value: bson.A{
													3,
													6,
													9,
													12,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			bson.D{{Key: "$sort", Value: bson.D{{Key: "score", Value: -1}}}},
			bson.D{{Key: "$limit", Value: 15}},
		})

		if err != nil {
			fmt.Print(err)
		}
		defer out.Close(context.TODO())
		iter := 1

		var results []bson.M

		if err = out.All(context.TODO(), &results); err != nil {
			log.Fatal(err)
		}
		for _, result := range results {
			dataStr := "<div class='py-0 my-0' style='display: inline-flex;'><p class='px-2 m-0' style='position: absolute; left: 1%;'>" + fmt.Sprintf("%d", iter) + ".)&nbsp;&nbsp;</p><p class='px-1 m-0' style='text-align: center; position: absolute; left: 11%;'>" + result["username"].(string) + "</p><p class='px-2 mx-3' style='text-align: center; position: absolute; left: 41%;'>" + fmt.Sprint(result["bonus_points"].(int32)) + "</p><p class='px-2 mx-3' style='text-align: center; position: absolute; left: 56%;'>" + fmt.Sprint(result["level"].(int32)) + "</p><p class='px-2 mx-3' style='text-align: center; position: absolute; left: 76%;'>" + strings.Split(result["org_id"].(string), "_")[0] + "</p></div><br/>"
			iter++
			w.Write([]byte(dataStr))
			if iter == 20 {
				return
			}
		}
	}
}
func UpdateStackerzScoreHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	bs, _ := io.ReadAll(r.Body)
	type postBody struct {
		Username    string `json:"username"`
		BonusPoints int    `json:"bonus_points"`
		Level       int    `json:"level"`
	}
	var postData postBody
	marsherr := json.Unmarshal(bs, &postData)
	if marsherr != nil {
		fmt.Println(marsherr)
	}
	_, inserr := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.stack_leaderboard(\"username\", \"bonus_points\", \"level\") values('%s', %d, %d)", postData.Username, postData.BonusPoints, postData.Level))
	if inserr != nil {
		activityStr := fmt.Sprintf("could not update stackerz leaderboard for %s", currentUserFromSession)
		globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.errlog(\"errmessage\", \"createdon\", \"activity\") values(substr('%s',0,105), '%s', substr('%s',0,105));", inserr, time.Now().In(globalvars.NyLoc).Format(time.DateTime), activityStr))
	}
	globalvars.Leaderboardcoll.InsertOne(context.TODO(), bson.M{"org_id": globalvars.OrgId, "game": "stackerz", "bonus_points": postData.BonusPoints, "level": postData.Level, "username": postData.Username, "createdOn": time.Now()})
}
func GetPersonalCatchitLeaderboardHandler(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, currentUserFromSession, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	output, outerr := globalvars.Db.Query(fmt.Sprintf("select username, score from tfldata.catchitleaderboard where username='%s' order by score desc limit 20;", currentUserFromSession))
	if outerr != nil {
		fmt.Println(outerr)
	}
	defer output.Close()
	iter := 1
	for output.Next() {
		var username string
		var score string
		scnerr := output.Scan(&username, &score)
		if scnerr != nil {
			fmt.Println(scnerr)
		}
		dataStr := "<div class='py-0 my-0' style='display: inline-flex;'><p class='px-2 m-0' style='position: absolute; left: 2%;'>" + fmt.Sprintf("%d", iter) + ".)&nbsp;&nbsp;</p><p class='px-2 m-0' style='text-align: center; position: absolute; left: 15%;'>" + username + "</p><p class='px-2 m-0' style='text-align: center; position: absolute; left: 65%;'>" + score + "</p></div><br/>"
		iter++
		w.Write([]byte(dataStr))
	}
}
func GetCatchitLeaderboardHandler(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.URL.Query().Get("leaderboardType") == "family" {
		output, outerr := globalvars.Db.Query("select username, score from tfldata.catchitleaderboard order by score desc limit 20;")
		if outerr != nil {
			fmt.Println(outerr)
		}
		defer output.Close()
		iter := 1
		for output.Next() {
			var username string
			var score string
			scnerr := output.Scan(&username, &score)
			if scnerr != nil {
				fmt.Println(scnerr)
			}
			dataStr := "<div class='py-0 my-0' style='display: inline-flex;'><p class='px-2 m-0' style='position: absolute; left: 2%;'>" + fmt.Sprintf("%d", iter) + ".)&nbsp;&nbsp;</p><p class='px-2 m-0' style='text-align: center; position: absolute; left: 15%;'>" + username + "</p><p class='px-2 m-0' style='text-align: center; position: absolute; left: 65%;'>" + score + "</p></div><br/>"
			iter++
			w.Write([]byte(dataStr))
		}
	} else if r.URL.Query().Get("leaderboardType") == "global" {
		eventYearConverted, _ := strconv.Atoi(r.URL.Query().Get("eventYear"))
		eventPeriodConverted, _ := strconv.Atoi(r.URL.Query().Get("eventPeriod"))
		var startPeriodMonth int
		var endPeriodMonth int
		switch eventPeriodConverted {
		case 1:
			startPeriodMonth = 0
			endPeriodMonth = 4
		case 2:
			startPeriodMonth = 3
			endPeriodMonth = 7
		case 3:
			startPeriodMonth = 6
			endPeriodMonth = 10
		case 4:
			startPeriodMonth = 9
			endPeriodMonth = 13
		}
		out, err := globalvars.Leaderboardcoll.Aggregate(context.TODO(), bson.A{
			bson.D{{Key: "$match", Value: bson.D{{Key: "game", Value: "catchit"}}}},
			bson.D{
				{Key: "$set",
					Value: bson.D{
						{Key: "year",
							Value: bson.D{
								{Key: "$abs",
									Value: bson.D{
										{Key: "$subtract",
											Value: bson.A{
												2020,
												bson.D{{Key: "$year", Value: "$createdOn"}},
											},
										},
									},
								},
							},
						},
						{Key: "month", Value: bson.D{{Key: "$month", Value: "$createdOn"}}},
						{Key: "day", Value: bson.D{
							{Key: "$subtract",
								Value: bson.A{
									bson.D{{Key: "$dayOfMonth", Value: "$createdOn"}},
									1,
								},
							},
						},
						},
					},
				},
			},
			bson.D{
				{Key: "$match",
					Value: bson.D{
						{Key: "year", Value: eventYearConverted},
						{Key: "month",
							Value: bson.D{
								{Key: "$gt", Value: startPeriodMonth},
								{Key: "$lt", Value: endPeriodMonth},
							},
						},
						{Key: "$and",
							Value: bson.A{
								bson.D{{Key: "day", Value: bson.D{{Key: "$lt", Value: 22}}}},
								bson.D{
									{Key: "month",
										Value: bson.D{
											{Key: "$ne",
												Value: bson.A{
													3,
													6,
													9,
													12,
												},
											},
										},
									},
								},
							}},
					},
				},
			},
			bson.D{{Key: "$sort", Value: bson.D{{Key: "score", Value: -1}}}},
			bson.D{{Key: "$limit", Value: 15}},
		})

		if err != nil {
			fmt.Print(err)
		}
		defer out.Close(context.TODO())
		iter := 1

		var results []bson.M

		if err = out.All(context.TODO(), &results); err != nil {
			log.Fatal(err)
		}
		for _, result := range results {
			dataStr := "<div class='py-0 my-0' style='display: inline-flex;'><p class='px-2 m-0' style='position: absolute; left: 1%;'>" + fmt.Sprintf("%d", iter) + ".)&nbsp;&nbsp;</p><p class='px-1 m-0' style='text-align: center; position: absolute; left: 13%;'>" + result["username"].(string) + "</p><p class='px-2 mx-5' style='text-align: center; position: absolute; left: 40%;'>" + fmt.Sprint(result["score"].(int32)) + "</p><p class='px-2 mx-5' style='text-align: center; position: absolute; left: 65%;'>" + strings.Split(result["org_id"].(string), "_")[0] + "</p></div><br/>"
			iter++
			w.Write([]byte(dataStr))
		}

	}
}
func UpdateCatchitScoreHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	bs, _ := io.ReadAll(r.Body)
	type postBody struct {
		Username string `json:"username"`
		Score    int    `json:"score"`
	}
	var postData postBody
	marsherr := json.Unmarshal(bs, &postData)
	if marsherr != nil {
		fmt.Println(marsherr)
	}

	_, inserr := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.catchitleaderboard(\"username\", \"score\", \"createdon\") values('%s', '%d', now());", postData.Username, postData.Score))
	if inserr != nil {
		w.WriteHeader(http.StatusBadRequest)
	}
	globalvars.Leaderboardcoll.InsertOne(context.TODO(), bson.M{"org_id": globalvars.OrgId, "game": "catchit", "score": postData.Score, "username": postData.Username, "createdOn": time.Now()})

}
func GetLeaderboardHandler(w http.ResponseWriter, r *http.Request) {
	allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

	validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
	if !validBool || !allowOrDeny {
		w.Header().Set("HX-Retarget", "window")
		w.Header().Set("HX-Trigger", h)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.URL.Query().Get("leaderboardType") == "family" {
		output, outerr := globalvars.Db.Query("select username, score from tfldata.ss_leaderboard order by score desc limit 20;")
		if outerr != nil {
			fmt.Println(outerr)
		}
		defer output.Close()
		iter := 1
		for output.Next() {
			var username string
			var score string
			scnerr := output.Scan(&username, &score)
			if scnerr != nil {
				fmt.Println(scnerr)
			}
			// dataStr := "<div class='py-0 my-0' style='display: inline-flex;'><p class='px-2 m-0'>" + fmt.Sprintf("%d", iter) + "</p><p class='px-2 m-0' style='text-align: center;'>" + username + " - " + score + "</p></div><br/>"
			dataStr := "<div class='py-0 my-0' style='display: inline-flex;'><p class='px-2 m-0' style='position: absolute; left: 2%;'>" + fmt.Sprintf("%d", iter) + ".)&nbsp;&nbsp;</p><p class='px-2 m-0' style='text-align: center; position: absolute; left: 15%;'>" + username + "</p><p class='px-2 m-0' style='text-align: center; position: absolute; left: 65%;'>" + score + "</p></div><br/>"
			iter++
			w.Write([]byte(dataStr))
		}
	} else if r.URL.Query().Get("leaderboardType") == "global" {
		eventYearConverted, _ := strconv.Atoi(r.URL.Query().Get("eventYear"))
		eventPeriodConverted, _ := strconv.Atoi(r.URL.Query().Get("eventPeriod"))
		var startPeriodMonth int
		var endPeriodMonth int
		switch eventPeriodConverted {
		case 1:
			startPeriodMonth = 0
			endPeriodMonth = 4
		case 2:
			startPeriodMonth = 3
			endPeriodMonth = 7
		case 3:
			startPeriodMonth = 6
			endPeriodMonth = 10
		case 4:
			startPeriodMonth = 9
			endPeriodMonth = 13
		}
		out, err := globalvars.Leaderboardcoll.Aggregate(context.TODO(), bson.A{
			bson.D{{Key: "$match", Value: bson.D{{Key: "game", Value: "simple_shades"}}}},
			bson.D{
				{Key: "$set",
					Value: bson.D{
						{Key: "year",
							Value: bson.D{
								{Key: "$abs",
									Value: bson.D{
										{Key: "$subtract",
											Value: bson.A{
												2020,
												bson.D{{Key: "$year", Value: "$createdOn"}},
											},
										},
									},
								},
							},
						},
						{Key: "month", Value: bson.D{{Key: "$month", Value: "$createdOn"}}},
						{Key: "day", Value: bson.D{
							{Key: "$subtract",
								Value: bson.A{
									bson.D{{Key: "$dayOfWeek", Value: "$createdOn"}},
									1,
								},
							},
						},
						},
					},
				},
			},
			bson.D{
				{Key: "$match",
					Value: bson.D{
						{Key: "year", Value: eventYearConverted},
						{Key: "month",
							Value: bson.D{
								{Key: "$gt", Value: startPeriodMonth},
								{Key: "$lt", Value: endPeriodMonth},
							},
						},
						{Key: "$and",
							Value: bson.A{
								bson.D{{Key: "day", Value: bson.D{{Key: "$lt", Value: 22}}}},
								bson.D{
									{Key: "month",
										Value: bson.D{
											{Key: "$ne",
												Value: bson.A{
													3,
													6,
													9,
													12,
												},
											},
										},
									},
								},
							}},
					},
				},
			},
			bson.D{{Key: "$sort", Value: bson.D{{Key: "score", Value: -1}}}},
			bson.D{{Key: "$limit", Value: 15}},
		})

		if err != nil {
			fmt.Print(err)
		}
		defer out.Close(context.TODO())
		iter := 1

		var results []bson.M

		if err = out.All(context.TODO(), &results); err != nil {
			log.Fatal(err)
		}
		for _, result := range results {
			dataStr := "<div class='py-0 my-0' style='display: inline-flex;'><p class='px-2 m-0' style='position: absolute; left: 1%;'>" + fmt.Sprintf("%d", iter) + ".)&nbsp;&nbsp;</p><p class='px-1 m-0' style='text-align: center; position: absolute; left: 13%;'>" + result["username"].(string) + "</p><p class='px-2 mx-5' style='text-align: center; position: absolute; left: 40%;'>" + fmt.Sprint(result["score"].(int32)) + "</p><p class='px-2 mx-5' style='text-align: center; position: absolute; left: 55%;'>" + strings.Split(result["org_id"].(string), "_")[0] + "</p></div><br/>"
			iter++
			w.Write([]byte(dataStr))
		}
	}
}
func UpdateSimpleShadesScoreHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	bs, _ := io.ReadAll(r.Body)
	type postBody struct {
		Username string `json:"username"`
		Score    int    `json:"score"`
	}
	var postData postBody
	errmarsh := json.Unmarshal(bs, &postData)
	if errmarsh != nil {
		fmt.Println(errmarsh)
	}

	_, inserr := globalvars.Db.Exec(fmt.Sprintf("insert into tfldata.ss_leaderboard(\"username\", \"score\") values('%s', '%d');", postData.Username, postData.Score))
	if inserr != nil {
		w.WriteHeader(http.StatusBadRequest)
	}
	globalvars.Leaderboardcoll.InsertOne(context.TODO(), bson.M{"org_id": globalvars.OrgId, "game": "simple_shades", "score": postData.Score, "username": postData.Username, "createdOn": time.Now()})

}
