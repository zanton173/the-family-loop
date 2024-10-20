--
-- PostgreSQL database dump
--

-- Dumped from database version 15.4
-- Dumped by pg_dump version 15.4

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: tfldata; Type: SCHEMA; Schema: -; Owner: tfldbrole
--

CREATE SCHEMA tfldata;


ALTER SCHEMA tfldata OWNER TO tfldbrole;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: calendar; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.calendar (
    id integer NOT NULL,
    start_date date,
    event_owner character varying(15),
    event_details character varying(220),
    event_title character varying(42),
    end_date date
);


ALTER TABLE tfldata.calendar OWNER TO tfldbrole;

--
-- Name: calendar_event_date_id_seq; Type: SEQUENCE; Schema: tfldata; Owner: tfldbrole
--

CREATE SEQUENCE tfldata.calendar_event_date_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE tfldata.calendar_event_date_id_seq OWNER TO tfldbrole;

--
-- Name: calendar_event_date_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.calendar_event_date_id_seq OWNED BY tfldata.calendar.id;


--
-- Name: calendar_rsvp; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.calendar_rsvp (
    id integer NOT NULL,
    username character varying(15),
    event_id integer,
    status character varying(5)
);


ALTER TABLE tfldata.calendar_rsvp OWNER TO tfldbrole;

--
-- Name: calendar_rsvp_id_seq; Type: SEQUENCE; Schema: tfldata; Owner: tfldbrole
--

CREATE SEQUENCE tfldata.calendar_rsvp_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE tfldata.calendar_rsvp_id_seq OWNER TO tfldbrole;

--
-- Name: calendar_rsvp_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.calendar_rsvp_id_seq OWNED BY tfldata.calendar_rsvp.id;


--
-- Name: catchitleaderboard; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.catchitleaderboard (
    id integer NOT NULL,
    username character varying(15),
    score integer,
    createdon timestamp without time zone
);


ALTER TABLE tfldata.catchitleaderboard OWNER TO tfldbrole;

--
-- Name: catchitleaderboard_id_seq; Type: SEQUENCE; Schema: tfldata; Owner: tfldbrole
--

CREATE SEQUENCE tfldata.catchitleaderboard_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE tfldata.catchitleaderboard_id_seq OWNER TO tfldbrole;

--
-- Name: catchitleaderboard_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.catchitleaderboard_id_seq OWNED BY tfldata.catchitleaderboard.id;


--
-- Name: comments; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.comments (
    post_id integer,
    comment character varying(280),
    author character varying(15),
    id integer NOT NULL,
    event_id integer
);


ALTER TABLE tfldata.comments OWNER TO tfldbrole;

--
-- Name: comments_id_seq; Type: SEQUENCE; Schema: tfldata; Owner: tfldbrole
--

CREATE SEQUENCE tfldata.comments_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE tfldata.comments_id_seq OWNER TO tfldbrole;

--
-- Name: comments_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.comments_id_seq OWNED BY tfldata.comments.id;


--
-- Name: errlog; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.errlog (
    id integer NOT NULL,
    errmessage character varying(420),
    createdon timestamp without time zone,
    activity character varying(106)
);


ALTER TABLE tfldata.errlog OWNER TO tfldbrole;

--
-- Name: errlog_id_seq; Type: SEQUENCE; Schema: tfldata; Owner: tfldbrole
--

CREATE SEQUENCE tfldata.errlog_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE tfldata.errlog_id_seq OWNER TO tfldbrole;

--
-- Name: errlog_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.errlog_id_seq OWNED BY tfldata.errlog.id;


--
-- Name: gchat; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.gchat (
    id integer NOT NULL,
    chat character varying(420),
    author character varying(15),
    createdon timestamp with time zone,
    thread character varying(32)
);


ALTER TABLE tfldata.gchat OWNER TO tfldbrole;

--
-- Name: gchat_id_seq; Type: SEQUENCE; Schema: tfldata; Owner: tfldbrole
--

CREATE SEQUENCE tfldata.gchat_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE tfldata.gchat_id_seq OWNER TO tfldbrole;

--
-- Name: gchat_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.gchat_id_seq OWNED BY tfldata.gchat.id;


--
-- Name: inclog; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.inclog (
    id integer NOT NULL,
    ip_addr character varying(15)
);


ALTER TABLE tfldata.inclog OWNER TO tfldbrole;

--
-- Name: inclog_id_seq; Type: SEQUENCE; Schema: tfldata; Owner: tfldbrole
--

CREATE SEQUENCE tfldata.inclog_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE tfldata.inclog_id_seq OWNER TO tfldbrole;

--
-- Name: inclog_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.inclog_id_seq OWNED BY tfldata.inclog.id;


--
-- Name: invite_sent_requests; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.invite_sent_requests (
    from_admin character varying(13),
    to_user_email character varying(255),
    to_user_first_name character varying(35),
    hassent boolean
);


ALTER TABLE tfldata.invite_sent_requests OWNER TO tfldbrole;

--
-- Name: pchat; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.pchat (
    id integer NOT NULL,
    message character varying(420),
    from_user character varying(15),
    to_user character varying(15),
    reaction character varying(9),
    createdon timestamp with time zone
);


ALTER TABLE tfldata.pchat OWNER TO tfldbrole;

--
-- Name: pchat_id_seq; Type: SEQUENCE; Schema: tfldata; Owner: tfldbrole
--

CREATE SEQUENCE tfldata.pchat_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE tfldata.pchat_id_seq OWNER TO tfldbrole;

--
-- Name: pchat_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.pchat_id_seq OWNED BY tfldata.pchat.id;


--
-- Name: pong_game_lobby; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.pong_game_lobby (
    player_username character varying(13),
    id integer NOT NULL
);


ALTER TABLE tfldata.pong_game_lobby OWNER TO tfldbrole;

--
-- Name: pong_game_lobby_id_seq; Type: SEQUENCE; Schema: tfldata; Owner: tfldbrole
--

CREATE SEQUENCE tfldata.pong_game_lobby_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE tfldata.pong_game_lobby_id_seq OWNER TO tfldbrole;

--
-- Name: pong_game_lobby_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.pong_game_lobby_id_seq OWNED BY tfldata.pong_game_lobby.id;


--
-- Name: pong_game_state; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.pong_game_state (
    id integer NOT NULL,
    playerone character varying(13),
    playertwo character varying(13),
    playeroneconnected boolean,
    playertwoconnected boolean
);


ALTER TABLE tfldata.pong_game_state OWNER TO tfldbrole;

--
-- Name: pong_game_state_id_seq; Type: SEQUENCE; Schema: tfldata; Owner: tfldbrole
--

CREATE SEQUENCE tfldata.pong_game_state_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE tfldata.pong_game_state_id_seq OWNER TO tfldbrole;

--
-- Name: pong_game_state_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.pong_game_state_id_seq OWNED BY tfldata.pong_game_state.id;


--
-- Name: pong_match_history; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.pong_match_history (
    playeronename character varying(13),
    playertwoname character varying(13),
    playeronescore character varying(2),
    playertwoscore character varying(2),
    matchid integer,
    createdon timestamp without time zone
);


ALTER TABLE tfldata.pong_match_history OWNER TO tfldbrole;

--
-- Name: postfiles; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.postfiles (
    id integer NOT NULL,
    file_name character varying(64),
    file_type character varying(64),
    post_files_key uuid
);


ALTER TABLE tfldata.postfiles OWNER TO tfldbrole;

--
-- Name: postfiles_id_seq; Type: SEQUENCE; Schema: tfldata; Owner: tfldbrole
--

CREATE SEQUENCE tfldata.postfiles_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE tfldata.postfiles_id_seq OWNER TO tfldbrole;

--
-- Name: postfiles_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.postfiles_id_seq OWNED BY tfldata.postfiles.id;


--
-- Name: posts; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.posts (
    id integer NOT NULL,
    title character varying(128),
    description character varying(420),
    author character varying(15),
    post_files_key uuid,
    createdon timestamp without time zone,
    available boolean
);


ALTER TABLE tfldata.posts OWNER TO tfldbrole;

--
-- Name: posts_id_seq; Type: SEQUENCE; Schema: tfldata; Owner: tfldbrole
--

CREATE SEQUENCE tfldata.posts_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE tfldata.posts_id_seq OWNER TO tfldbrole;

--
-- Name: posts_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.posts_id_seq OWNED BY tfldata.posts.id;


--
-- Name: reactions; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.reactions (
    id integer NOT NULL,
    post_id integer,
    gchat_id integer,
    author character varying(15),
    reaction character varying(9)
);


ALTER TABLE tfldata.reactions OWNER TO tfldbrole;

--
-- Name: reactions_id_seq; Type: SEQUENCE; Schema: tfldata; Owner: tfldbrole
--

CREATE SEQUENCE tfldata.reactions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE tfldata.reactions_id_seq OWNER TO tfldbrole;

--
-- Name: reactions_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.reactions_id_seq OWNED BY tfldata.reactions.id;


--
-- Name: sent_notification_log; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.sent_notification_log (
    id integer NOT NULL,
    notification_result character varying(128),
    createdon timestamp without time zone
);


ALTER TABLE tfldata.sent_notification_log OWNER TO tfldbrole;

--
-- Name: sent_notification_log_id_seq; Type: SEQUENCE; Schema: tfldata; Owner: tfldbrole
--

CREATE SEQUENCE tfldata.sent_notification_log_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE tfldata.sent_notification_log_id_seq OWNER TO tfldbrole;

--
-- Name: sent_notification_log_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.sent_notification_log_id_seq OWNED BY tfldata.sent_notification_log.id;


--
-- Name: ss_leaderboard; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.ss_leaderboard (
    id integer NOT NULL,
    username character varying(15),
    score integer,
    createdon timestamp without time zone
);


ALTER TABLE tfldata.ss_leaderboard OWNER TO tfldbrole;

--
-- Name: ss_leaderboard_id_seq; Type: SEQUENCE; Schema: tfldata; Owner: tfldbrole
--

CREATE SEQUENCE tfldata.ss_leaderboard_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE tfldata.ss_leaderboard_id_seq OWNER TO tfldbrole;

--
-- Name: ss_leaderboard_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.ss_leaderboard_id_seq OWNED BY tfldata.ss_leaderboard.id;


--
-- Name: stack_leaderboard; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.stack_leaderboard (
    id integer NOT NULL,
    username character varying(15),
    bonus_points integer,
    level integer,
    createdon timestamp without time zone
);


ALTER TABLE tfldata.stack_leaderboard OWNER TO tfldbrole;

--
-- Name: stack_leaderboard_id_seq; Type: SEQUENCE; Schema: tfldata; Owner: tfldbrole
--

CREATE SEQUENCE tfldata.stack_leaderboard_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE tfldata.stack_leaderboard_id_seq OWNER TO tfldbrole;

--
-- Name: stack_leaderboard_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.stack_leaderboard_id_seq OWNED BY tfldata.stack_leaderboard.id;


--
-- Name: threads; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.threads (
    thread character varying(32) NOT NULL,
    threadauthor character varying(15),
    createdon timestamp without time zone
);


ALTER TABLE tfldata.threads OWNER TO tfldbrole;

--
-- Name: timecapsule; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.timecapsule (
    username character varying(15),
    available_on date,
    tcname character varying(18),
    createdon date,
    tcfilename character varying(59),
    waspurchased boolean,
    wasearlyaccesspurchased boolean,
    yearstostore integer,
    wasrequested boolean,
    wasdownloaded boolean
);


ALTER TABLE tfldata.timecapsule OWNER TO tfldbrole;

--
-- Name: users; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.users (
    id integer NOT NULL,
    username character varying(15),
    password character varying(4096),
    orgid character varying(256),
    pfp_name character varying(128),
    session_token uuid,
    email character varying(64),
    fcm_registration_id character varying(168),
    gchat_bg_theme character varying(65),
    last_sign_on timestamp without time zone,
    gchat_order_option boolean,
    cf_domain_name character varying(30),
    is_admin boolean,
    last_pass_reset timestamp without time zone,
    mytz character varying(30),
    last_viewed_pchat character varying(15),
    last_viewed_gchat character varying(32),
    is_paying_subscriber boolean,
    wix_member_id character varying(40),
    isloopowner boolean
);


ALTER TABLE tfldata.users OWNER TO tfldbrole;

--
-- Name: users_id_seq; Type: SEQUENCE; Schema: tfldata; Owner: tfldbrole
--

CREATE SEQUENCE tfldata.users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE tfldata.users_id_seq OWNER TO tfldbrole;

--
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.users_id_seq OWNED BY tfldata.users.id;


--
-- Name: users_to_threads; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.users_to_threads (
    username character varying(128),
    thread character varying(32),
    is_subscribed boolean
);


ALTER TABLE tfldata.users_to_threads OWNER TO tfldbrole;

--
-- Name: calendar id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.calendar ALTER COLUMN id SET DEFAULT nextval('tfldata.calendar_event_date_id_seq'::regclass);


--
-- Name: calendar_rsvp id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.calendar_rsvp ALTER COLUMN id SET DEFAULT nextval('tfldata.calendar_rsvp_id_seq'::regclass);


--
-- Name: catchitleaderboard id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.catchitleaderboard ALTER COLUMN id SET DEFAULT nextval('tfldata.catchitleaderboard_id_seq'::regclass);


--
-- Name: comments id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.comments ALTER COLUMN id SET DEFAULT nextval('tfldata.comments_id_seq'::regclass);


--
-- Name: errlog id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.errlog ALTER COLUMN id SET DEFAULT nextval('tfldata.errlog_id_seq'::regclass);


--
-- Name: gchat id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.gchat ALTER COLUMN id SET DEFAULT nextval('tfldata.gchat_id_seq'::regclass);


--
-- Name: inclog id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.inclog ALTER COLUMN id SET DEFAULT nextval('tfldata.inclog_id_seq'::regclass);


--
-- Name: pchat id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.pchat ALTER COLUMN id SET DEFAULT nextval('tfldata.pchat_id_seq'::regclass);


--
-- Name: pong_game_lobby id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.pong_game_lobby ALTER COLUMN id SET DEFAULT nextval('tfldata.pong_game_lobby_id_seq'::regclass);


--
-- Name: pong_game_state id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.pong_game_state ALTER COLUMN id SET DEFAULT nextval('tfldata.pong_game_state_id_seq'::regclass);


--
-- Name: postfiles id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.postfiles ALTER COLUMN id SET DEFAULT nextval('tfldata.postfiles_id_seq'::regclass);


--
-- Name: posts id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.posts ALTER COLUMN id SET DEFAULT nextval('tfldata.posts_id_seq'::regclass);


--
-- Name: reactions id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.reactions ALTER COLUMN id SET DEFAULT nextval('tfldata.reactions_id_seq'::regclass);


--
-- Name: sent_notification_log id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.sent_notification_log ALTER COLUMN id SET DEFAULT nextval('tfldata.sent_notification_log_id_seq'::regclass);


--
-- Name: ss_leaderboard id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.ss_leaderboard ALTER COLUMN id SET DEFAULT nextval('tfldata.ss_leaderboard_id_seq'::regclass);


--
-- Name: stack_leaderboard id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.stack_leaderboard ALTER COLUMN id SET DEFAULT nextval('tfldata.stack_leaderboard_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.users ALTER COLUMN id SET DEFAULT nextval('tfldata.users_id_seq'::regclass);


--
-- Name: calendar calendar_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.calendar
    ADD CONSTRAINT calendar_pkey PRIMARY KEY (id);


--
-- Name: calendar_rsvp calendar_rsvp_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.calendar_rsvp
    ADD CONSTRAINT calendar_rsvp_pkey PRIMARY KEY (id);


--
-- Name: calendar_rsvp calendar_rsvp_username_event_id_key; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.calendar_rsvp
    ADD CONSTRAINT calendar_rsvp_username_event_id_key UNIQUE (username, event_id);


--
-- Name: catchitleaderboard catchitleaderboard_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.catchitleaderboard
    ADD CONSTRAINT catchitleaderboard_pkey PRIMARY KEY (id);


--
-- Name: comments comments_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.comments
    ADD CONSTRAINT comments_pkey PRIMARY KEY (id);


--
-- Name: errlog errlog_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.errlog
    ADD CONSTRAINT errlog_pkey PRIMARY KEY (id);


--
-- Name: gchat gchat_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.gchat
    ADD CONSTRAINT gchat_pkey PRIMARY KEY (id);


--
-- Name: inclog inclog_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.inclog
    ADD CONSTRAINT inclog_pkey PRIMARY KEY (id);


--
-- Name: pchat pchat_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.pchat
    ADD CONSTRAINT pchat_pkey PRIMARY KEY (id);


--
-- Name: pong_game_lobby pong_game_lobby_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.pong_game_lobby
    ADD CONSTRAINT pong_game_lobby_pkey PRIMARY KEY (id);


--
-- Name: pong_game_state pong_game_state_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.pong_game_state
    ADD CONSTRAINT pong_game_state_pkey PRIMARY KEY (id);


--
-- Name: pong_match_history pong_match_history_matchid_key; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.pong_match_history
    ADD CONSTRAINT pong_match_history_matchid_key UNIQUE (matchid);


--
-- Name: postfiles postfiles_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.postfiles
    ADD CONSTRAINT postfiles_pkey PRIMARY KEY (id);


--
-- Name: posts posts_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.posts
    ADD CONSTRAINT posts_pkey PRIMARY KEY (id);


--
-- Name: reactions reactions_gchat_id_author_key; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.reactions
    ADD CONSTRAINT reactions_gchat_id_author_key UNIQUE (gchat_id, author);


--
-- Name: reactions reactions_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.reactions
    ADD CONSTRAINT reactions_pkey PRIMARY KEY (id);


--
-- Name: reactions reactions_post_id_author_key; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.reactions
    ADD CONSTRAINT reactions_post_id_author_key UNIQUE (post_id, author);


--
-- Name: sent_notification_log sent_notification_log_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.sent_notification_log
    ADD CONSTRAINT sent_notification_log_pkey PRIMARY KEY (id);


--
-- Name: ss_leaderboard ss_leaderboard_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.ss_leaderboard
    ADD CONSTRAINT ss_leaderboard_pkey PRIMARY KEY (id);


--
-- Name: stack_leaderboard stack_leaderboard_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.stack_leaderboard
    ADD CONSTRAINT stack_leaderboard_pkey PRIMARY KEY (id);


--
-- Name: threads threads_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.threads
    ADD CONSTRAINT threads_pkey PRIMARY KEY (thread);


--
-- Name: pong_game_lobby unique_player; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.pong_game_lobby
    ADD CONSTRAINT unique_player UNIQUE (player_username);


--
-- Name: pong_game_state uniqueplayers; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.pong_game_state
    ADD CONSTRAINT uniqueplayers UNIQUE (playerone, playertwo);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: users_to_threads users_to_threads_username_thread_key; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.users_to_threads
    ADD CONSTRAINT users_to_threads_username_thread_key UNIQUE (username, thread);


--
-- Name: users users_username_key; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.users
    ADD CONSTRAINT users_username_key UNIQUE (username);


--
-- Name: calendar_rsvp_tbl_event_id_idx; Type: INDEX; Schema: tfldata; Owner: tfldbrole
--

CREATE INDEX calendar_rsvp_tbl_event_id_idx ON tfldata.calendar_rsvp USING btree (event_id);


--
-- Name: calendar_rsvp_tbl_username_idx; Type: INDEX; Schema: tfldata; Owner: tfldbrole
--

CREATE INDEX calendar_rsvp_tbl_username_idx ON tfldata.calendar_rsvp USING btree (username);


--
-- Name: comments_tbl_event_id_idx; Type: INDEX; Schema: tfldata; Owner: tfldbrole
--

CREATE INDEX comments_tbl_event_id_idx ON tfldata.comments USING btree (event_id);


--
-- Name: comments_tbl_post_id_idx; Type: INDEX; Schema: tfldata; Owner: tfldbrole
--

CREATE INDEX comments_tbl_post_id_idx ON tfldata.comments USING btree (post_id);


--
-- Name: gchat_tbl_author_idx; Type: INDEX; Schema: tfldata; Owner: tfldbrole
--

CREATE INDEX gchat_tbl_author_idx ON tfldata.gchat USING btree (author);


--
-- Name: gchat_tbl_thread_idx; Type: INDEX; Schema: tfldata; Owner: tfldbrole
--

CREATE INDEX gchat_tbl_thread_idx ON tfldata.gchat USING btree (thread);


--
-- Name: invite_sent_requests_to_user_email_from_admin_idx; Type: INDEX; Schema: tfldata; Owner: tfldbrole
--

CREATE INDEX invite_sent_requests_to_user_email_from_admin_idx ON tfldata.invite_sent_requests USING btree (to_user_email, from_admin);


--
-- Name: pchat_tbl_from_user_idx; Type: INDEX; Schema: tfldata; Owner: tfldbrole
--

CREATE INDEX pchat_tbl_from_user_idx ON tfldata.pchat USING btree (from_user);


--
-- Name: pchat_tbl_to_user_idx; Type: INDEX; Schema: tfldata; Owner: tfldbrole
--

CREATE INDEX pchat_tbl_to_user_idx ON tfldata.pchat USING btree (to_user);


--
-- Name: pong_match_history_current_user_idx01; Type: INDEX; Schema: tfldata; Owner: tfldbrole
--

CREATE INDEX pong_match_history_current_user_idx01 ON tfldata.pong_match_history USING btree (playeronename);


--
-- Name: pong_match_history_current_user_two_idx01; Type: INDEX; Schema: tfldata; Owner: tfldbrole
--

CREATE INDEX pong_match_history_current_user_two_idx01 ON tfldata.pong_match_history USING btree (playertwoname);


--
-- Name: postfiles_file_name_idx; Type: INDEX; Schema: tfldata; Owner: tfldbrole
--

CREATE INDEX postfiles_file_name_idx ON tfldata.postfiles USING btree (file_name);


--
-- Name: postfiles_tbl_post_files_key_idx; Type: INDEX; Schema: tfldata; Owner: tfldbrole
--

CREATE INDEX postfiles_tbl_post_files_key_idx ON tfldata.postfiles USING btree (post_files_key);


--
-- Name: posts_tbl_author_idx01; Type: INDEX; Schema: tfldata; Owner: tfldbrole
--

CREATE INDEX posts_tbl_author_idx01 ON tfldata.posts USING btree (author text_pattern_ops);


--
-- Name: posts_tbl_descr_idx01; Type: INDEX; Schema: tfldata; Owner: tfldbrole
--

CREATE INDEX posts_tbl_descr_idx01 ON tfldata.posts USING btree (description text_pattern_ops);


--
-- Name: posts_tbl_title_idx01; Type: INDEX; Schema: tfldata; Owner: tfldbrole
--

CREATE INDEX posts_tbl_title_idx01 ON tfldata.posts USING btree (title text_pattern_ops);


--
-- Name: utt_tbl_thread_idx; Type: INDEX; Schema: tfldata; Owner: tfldbrole
--

CREATE INDEX utt_tbl_thread_idx ON tfldata.users_to_threads USING btree (thread);


--
-- PostgreSQL database dump complete
--

