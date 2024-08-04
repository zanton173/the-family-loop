package main

import (
	"context"
	"log"
	"net/http"
	globalfunctions "tfl/functions"
	pages "tfl/handlers"
	adminhandler "tfl/handlers/admindash"
	authhandler "tfl/handlers/auth"
	calendarhandler "tfl/handlers/calendar"
	chathandler "tfl/handlers/chats"
	cshandler "tfl/handlers/customersupport"
	gameshandler "tfl/handlers/games"
	postshandler "tfl/handlers/posts"
	tchandler "tfl/handlers/timecapsule"
	userdatahandler "tfl/handlers/userdata"
	wixhandler "tfl/handlers/wix"
	globalvars "tfl/vars"

	_ "image/png"

	_ "github.com/lib/pq"
)

func main() {

	globalfunctions.InitalizeAll()
	// favicon
	faviconHandler := func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "assets/favicon.ico")
	}
	http.HandleFunc("/favicon.ico", faviconHandler)
	serviceWorkerHandler := func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "firebase-messaging-sw.js")
	}
	http.HandleFunc("/firebase-messaging-sw.js", serviceWorkerHandler)
	// Connect to database
	globalvars.Db.SetMaxIdleConns(25)
	defer globalvars.Db.Close()
	defer globalvars.MongoDb.Disconnect(context.TODO())

	validateEndpointForWixHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		validBool := globalfunctions.ValidateWebhookJWTToken(globalvars.JwtSignKey, r)
		if !validBool {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if globalvars.OrgId != r.URL.Query().Get("orgid") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte("true"))
	}

	/* NOT USING THIS RIGHT NOW */
	/*refreshTokenHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		allowOrDeny, _, h :=  globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		jwt.Parse(jwtCookie.Value, func(jwtToken *jwt.Token) (interface{}, error) {
			timeTilExp, _ := jwtToken.Claims.GetExpirationTime()
			if time.Until(timeTilExp.Time) < 24*time.Hour {
				globalfunctions.GenerateLoginJWT(r.URL.Query().Get("usersession"), w, r, jwtCookie.Value)

			}
			return []byte(globalvars.JwtSignKey), nil
		}, jwt.WithValidMethods([]string{"HS256"}))

	}*/
	validateJWTHandler := func(w http.ResponseWriter, r *http.Request) {
		allowOrDeny, _, h := globalfunctions.ValidateCurrentSessionId(globalvars.Db, r)

		validBool := globalfunctions.ValidateJWTToken(globalvars.JwtSignKey, r)
		if !validBool || !allowOrDeny {
			w.Header().Set("HX-Retarget", "window")
			w.Header().Set("HX-Trigger", h)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

	}
	deleteJWTHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		http.SetCookie(w, &http.Cookie{
			Name:     "backendauth",
			Value:    "",
			MaxAge:   0,
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
			Path:     "/",
		})
		//globalvars.Db.Exec(fmt.Sprintf("update tfldata.users set fcm_registration_id=null where username='%s';", r.URL.Query().Get("user")))
	}

	healthCheckHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("true"))
	}

	http.HandleFunc("/", pages.PagesHandler)
	/* posts handlers */
	http.HandleFunc("/create-post", postshandler.CreatePostHandler)
	http.HandleFunc("/create-reaction-to-post", postshandler.CreatePostReactionHandler)
	http.HandleFunc("/create-comment", postshandler.CreateCommentHandler)
	http.HandleFunc("/get-posts", postshandler.GetPostsHandler)
	http.HandleFunc("/delete-this-post", postshandler.DeleteThisPostHandler)
	http.HandleFunc("/get-selected-post", postshandler.GetSelectedPostsComments)
	http.HandleFunc("/get-posts-reactions", postshandler.GetPostsReactionsHandler)
	http.HandleFunc("/get-post-images", postshandler.GetPostImagesHandler)
	/* chat handlers */
	http.HandleFunc("/get-selected-chat", chathandler.GetSelectedChatHandler)
	http.HandleFunc("/get-selected-pchat", chathandler.GetSelectedPChatHandler)
	http.HandleFunc("/group-chat-messages", chathandler.GetGroupChatMessagesHandler)
	http.HandleFunc("/create-a-group-chat-message", chathandler.CreateGroupChatMessageHandler)
	http.HandleFunc("/del-thread", chathandler.DelThreadHandler)
	http.HandleFunc("/get-all-users-to-tag", chathandler.GetUsernamesToTagHandler)
	http.HandleFunc("/change-gchat-order-opt", chathandler.ChangeGchatOrderOptHandler)
	http.HandleFunc("/private-chat-messages", chathandler.GetPrivateChatMessagesHandler)
	http.HandleFunc("/create-a-private-chat-message", chathandler.CreatePrivatePChatMessageHandler)
	http.HandleFunc("/update-last-viewed-direct", chathandler.UpdateLastViewedPChatHandler)
	http.HandleFunc("/update-last-viewed-thread", chathandler.UpdateLastViewedThreadHandler)
	http.HandleFunc("/update-pchat-reaction", chathandler.UpdatePChatReactionHandler)
	http.HandleFunc("/current-pchat-reaction", chathandler.GetCurrentPChatReactionHandler)
	http.HandleFunc("/update-selected-chat", chathandler.UpdateSelectedChatHandler)
	http.HandleFunc("/delete-selected-chat", chathandler.DeleteSelectedChatHandler)
	http.HandleFunc("/update-selected-pchat", chathandler.UpdateSelectedPChatHandler)
	http.HandleFunc("/delete-selected-pchat", chathandler.DeleteSelectedPChatHandler)
	http.HandleFunc("/get-open-threads", chathandler.GetOpenThreadsHandler)
	http.HandleFunc("/get-users-chat", chathandler.GetUsersToChatToHandler)
	http.HandleFunc("/get-users-subscribed-threads", chathandler.GetUsersSubscribedThreadsHandler)
	http.HandleFunc("/change-if-notified-for-thread", chathandler.ChangeUserSubscriptionToThreadHandler)
	/* calendar handlers */
	http.HandleFunc("/get-events", calendarhandler.GetEventsHandler)
	http.HandleFunc("/get-event-comments", calendarhandler.GetSelectedEventsComments)
	http.HandleFunc("/create-event-comment", calendarhandler.CreateEventCommentHandler)
	http.HandleFunc("/create-event", calendarhandler.CreateEventHandler)
	http.HandleFunc("/update-rsvp-for-event", calendarhandler.UpdateRSVPForEventHandler)
	http.HandleFunc("/get-rsvp-data", calendarhandler.GetEventRSVPHandler)
	http.HandleFunc("/get-rsvp", calendarhandler.GetRSVPNotesHandler)
	http.HandleFunc("/delete-event", calendarhandler.DeleteEventHandler)
	/* Time Capsule handlers */
	http.HandleFunc("/create-new-tc", tchandler.CreateNewTimeCapsuleHandler)
	//http.HandleFunc("/get-my-time-capsules", getMyPurchasedTimeCapsulesHandler)
	http.HandleFunc("/get-my-purchased-time-capsules", tchandler.GetMyPurchasedTimeCapsulesHandler)
	http.HandleFunc("/get-my-notyetpurchased-time-capsules", tchandler.GetMyNotYetPurchasedTimeCapsulesHandler)
	http.HandleFunc("/get-my-available-time-capsules", tchandler.GetMyAvailableTimeCapsulesHandler)
	http.HandleFunc("/available-tc-was-downloaded", tchandler.AvailableTcWasDownloaded)
	http.HandleFunc("/get-my-tc-req-status", tchandler.GetMyTcRequestStatusHandler)
	http.HandleFunc("/initiate-tc-req-for-archive-file", tchandler.InitiateMyTCRestoreHandler)
	http.HandleFunc("/webhook-tc-early-access-payment-complete", tchandler.WixWebhookEarlyAccessPaymentCompleteHandler)
	http.HandleFunc("/webhook-tc-initial-payment-complete", tchandler.WixWebhookTCInitialPurchaseHandler)
	http.HandleFunc("/delete-my-tc", tchandler.DeleteMyTChandler)
	/* User data handlers */
	http.HandleFunc("/get-username-from-session", userdatahandler.GetSessionDataHandler)
	http.HandleFunc("/get-check-if-subscribed", userdatahandler.GetSubscribedHandler)
	http.HandleFunc("/create-subscription", userdatahandler.SubscriptionHandler)
	http.HandleFunc("/update-pfp", userdatahandler.UpdatePfpHandler)
	http.HandleFunc("/update-gchat-bg-theme", userdatahandler.UpdateChatThemeHandler)
	/* Customer Support handlers */
	http.HandleFunc("/create-issue", cshandler.CreateIssueHandler)
	http.HandleFunc("/get-my-customer-support-issues", cshandler.GetCustomerSupportIssuesHandler)
	http.HandleFunc("/get-issues-comments", cshandler.GetGHIssuesComments)
	http.HandleFunc("/create-issue-comment", cshandler.CreateGHIssueCommentHandler)
	/* Games handlers */
	http.HandleFunc("/get-leaderboard", gameshandler.GetLeaderboardHandler)
	http.HandleFunc("/update-simpleshades-score", gameshandler.UpdateSimpleShadesScoreHandler)
	http.HandleFunc("/get-stackerz-leaderboard", gameshandler.GetStackerzLeaderboardHandler)
	http.HandleFunc("/update-stackerz-score", gameshandler.UpdateStackerzScoreHandler)
	http.HandleFunc("/get-catchit-leaderboard", gameshandler.GetCatchitLeaderboardHandler)
	http.HandleFunc("/get-my-personal-score-catchit", gameshandler.GetPersonalCatchitLeaderboardHandler)
	http.HandleFunc("/update-catchit-score", gameshandler.UpdateCatchitScoreHandler)
	/* Wix handlers */
	http.HandleFunc("/wix-webhook-pricing-plan-changed", wixhandler.WixWebhookChangePlanHandler)
	http.HandleFunc("/wix-webhook-update-reg-user-paid-plan", wixhandler.RegUserPaidForPlanHandler)
	http.HandleFunc("/current-user-wix-subscription", wixhandler.GetCurrentUserSubPlan)
	http.HandleFunc("/get-admin-current-wix-sub-plan", wixhandler.GetAdminCurrentWixSubPlanHandler)
	http.HandleFunc("/send-reset-pass-wix-user", wixhandler.SendResetPassOnlyHandler)
	http.HandleFunc("/cancel-current-sub-regular-user", wixhandler.CancelCurrentSubRegUserHandler)
	http.HandleFunc("/cancel-plan-for-loop-owner", wixhandler.UpdateHostAdminPlanPaidForHandler)
	/* Admin dashboard handlers */
	http.HandleFunc("/admin-list-of-users", adminhandler.AdminGetListOfUsersHandler)
	http.HandleFunc("/admin-get-all-time-capsules", adminhandler.AdminGetAllTCHandler)
	http.HandleFunc("/admin-get-subscription-package", adminhandler.AdminGetSubPackageHandler)
	http.HandleFunc("/admin-delete-user", adminhandler.AdminDeleteUserHandler)
	http.HandleFunc("/admin-send-invite", adminhandler.AdminSendInviteHandler)
	/* Auth handlers */
	http.HandleFunc("/signup", authhandler.SignUpHandler)
	http.HandleFunc("/login", authhandler.LoginHandler)
	http.HandleFunc("/reset-password", authhandler.GetResetPasswordCodeHandler)
	http.HandleFunc("/reset-password-with-code", authhandler.ResetPasswordHandler)
	http.HandleFunc("/update-admin-pass", authhandler.UpdateAdminPassHandler)
	http.HandleFunc("/update-fcm-token", authhandler.UpdateFCMTokenHandler)
	// NOT USING THIS RIGHT NOW
	//http.HandleFunc("/refresh-token", refreshTokenHandler)
	http.HandleFunc("/delete-jwt", deleteJWTHandler)

	http.HandleFunc("/healthy-me-checky", healthCheckHandler)
	http.HandleFunc("/validate-endpoint-from-wix", validateEndpointForWixHandler)

	http.HandleFunc("/jwt-validation-endpoint", validateJWTHandler)

	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("css"))))
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("js"))))
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))

	log.Fatal(http.ListenAndServe(":80", nil))

}
