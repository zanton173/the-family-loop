--
-- PostgreSQL database dump
--

-- Dumped from database version 15.4
-- Dumped by pg_dump version 15.4

-- Started on 2023-12-07 04:46:23 EST

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
-- TOC entry 6 (class 2615 OID 16614)
-- Name: tfldata; Type: SCHEMA; Schema: -; Owner: tfldbrole
--

CREATE SCHEMA tfldata;


ALTER SCHEMA tfldata OWNER TO tfldbrole;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- TOC entry 216 (class 1259 OID 16615)
-- Name: calendar; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.calendar (
    id integer NOT NULL,
    start_date date,
    event_owner character varying(32),
    event_details character varying(220),
    event_title character varying(42)
);


ALTER TABLE tfldata.calendar OWNER TO tfldbrole;

--
-- TOC entry 217 (class 1259 OID 16618)
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
-- TOC entry 3713 (class 0 OID 0)
-- Dependencies: 217
-- Name: calendar_event_date_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.calendar_event_date_id_seq OWNED BY tfldata.calendar.id;


--
-- TOC entry 239 (class 1259 OID 16752)
-- Name: calendar_rsvp; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.calendar_rsvp (
    id integer NOT NULL,
    username character varying(128),
    event_id integer,
    status character varying(5)
);


ALTER TABLE tfldata.calendar_rsvp OWNER TO tfldbrole;

--
-- TOC entry 238 (class 1259 OID 16751)
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
-- TOC entry 3714 (class 0 OID 0)
-- Dependencies: 238
-- Name: calendar_rsvp_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.calendar_rsvp_id_seq OWNED BY tfldata.calendar_rsvp.id;


--
-- TOC entry 218 (class 1259 OID 16619)
-- Name: comments; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.comments (
    post_id integer,
    comment character varying(280),
    author character varying(32),
    id integer NOT NULL,
    event_id integer
);


ALTER TABLE tfldata.comments OWNER TO tfldbrole;

--
-- TOC entry 219 (class 1259 OID 16622)
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
-- TOC entry 3715 (class 0 OID 0)
-- Dependencies: 219
-- Name: comments_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.comments_id_seq OWNED BY tfldata.comments.id;


--
-- TOC entry 220 (class 1259 OID 16623)
-- Name: errlog; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.errlog (
    id integer NOT NULL,
    errmessage character varying(420),
    createdon timestamp without time zone
);


ALTER TABLE tfldata.errlog OWNER TO tfldbrole;

--
-- TOC entry 221 (class 1259 OID 16626)
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
-- TOC entry 3716 (class 0 OID 0)
-- Dependencies: 221
-- Name: errlog_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.errlog_id_seq OWNED BY tfldata.errlog.id;


--
-- TOC entry 222 (class 1259 OID 16627)
-- Name: gchat; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.gchat (
    id integer NOT NULL,
    chat character varying(420),
    author character varying(128),
    createdon timestamp without time zone,
    thread character varying(32)
);


ALTER TABLE tfldata.gchat OWNER TO tfldbrole;

--
-- TOC entry 223 (class 1259 OID 16632)
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
-- TOC entry 3717 (class 0 OID 0)
-- Dependencies: 223
-- Name: gchat_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.gchat_id_seq OWNED BY tfldata.gchat.id;


--
-- TOC entry 224 (class 1259 OID 16633)
-- Name: inclog; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.inclog (
    id integer NOT NULL,
    ip_addr character varying(15)
);


ALTER TABLE tfldata.inclog OWNER TO tfldbrole;

--
-- TOC entry 225 (class 1259 OID 16636)
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
-- TOC entry 3718 (class 0 OID 0)
-- Dependencies: 225
-- Name: inclog_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.inclog_id_seq OWNED BY tfldata.inclog.id;


--
-- TOC entry 230 (class 1259 OID 16673)
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
-- TOC entry 229 (class 1259 OID 16672)
-- Name: postfiles_id_seq; Type: SEQUENCE; Schema: tfldata; Owner: tfldbrole
--

CREATE SEQUENCE tfldata.postfiles_id_seq
    AS integer
    START WITH 83
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE tfldata.postfiles_id_seq OWNER TO tfldbrole;

--
-- TOC entry 3719 (class 0 OID 0)
-- Dependencies: 229
-- Name: postfiles_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.postfiles_id_seq OWNED BY tfldata.postfiles.id;


--
-- TOC entry 233 (class 1259 OID 16718)
-- Name: posts; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.posts (
    id integer NOT NULL,
    title character varying(128),
    description character varying(420),
    author character varying(128),
    post_files_key uuid
);


ALTER TABLE tfldata.posts OWNER TO tfldbrole;

--
-- TOC entry 231 (class 1259 OID 16696)
-- Name: posts_bkp; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.posts_bkp (
    id integer,
    title character varying(128),
    description character varying(420),
    file_name character varying(64),
    file_type character varying(64),
    author character varying(32)
);


ALTER TABLE tfldata.posts_bkp OWNER TO tfldbrole;

--
-- TOC entry 232 (class 1259 OID 16717)
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
-- TOC entry 3720 (class 0 OID 0)
-- Dependencies: 232
-- Name: posts_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.posts_id_seq OWNED BY tfldata.posts.id;


--
-- TOC entry 241 (class 1259 OID 16775)
-- Name: reactions; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.reactions (
    id integer NOT NULL,
    post_id integer,
    gchat_id integer,
    author character varying(128),
    reaction character varying(9)
);


ALTER TABLE tfldata.reactions OWNER TO tfldbrole;

--
-- TOC entry 240 (class 1259 OID 16774)
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
-- TOC entry 3721 (class 0 OID 0)
-- Dependencies: 240
-- Name: reactions_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.reactions_id_seq OWNED BY tfldata.reactions.id;


--
-- TOC entry 235 (class 1259 OID 16729)
-- Name: sent_notification_log; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.sent_notification_log (
    id integer NOT NULL,
    notification_result character varying(128),
    createdon timestamp without time zone
);


ALTER TABLE tfldata.sent_notification_log OWNER TO tfldbrole;

--
-- TOC entry 234 (class 1259 OID 16728)
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
-- TOC entry 3722 (class 0 OID 0)
-- Dependencies: 234
-- Name: sent_notification_log_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.sent_notification_log_id_seq OWNED BY tfldata.sent_notification_log.id;


--
-- TOC entry 226 (class 1259 OID 16637)
-- Name: sessions; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.sessions (
    session_token uuid,
    username character varying(32),
    expiry timestamp with time zone,
    ip_addr character varying(15)
);


ALTER TABLE tfldata.sessions OWNER TO tfldbrole;

--
-- TOC entry 237 (class 1259 OID 16736)
-- Name: ss_leaderboard; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.ss_leaderboard (
    id integer NOT NULL,
    username character varying(128),
    score integer,
    createdon timestamp without time zone
);


ALTER TABLE tfldata.ss_leaderboard OWNER TO tfldbrole;

--
-- TOC entry 236 (class 1259 OID 16735)
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
-- TOC entry 3723 (class 0 OID 0)
-- Dependencies: 236
-- Name: ss_leaderboard_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.ss_leaderboard_id_seq OWNED BY tfldata.ss_leaderboard.id;


--
-- TOC entry 243 (class 1259 OID 16867)
-- Name: stack_leaderboard; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.stack_leaderboard (
    id integer NOT NULL,
    username character varying(128),
    bonus_points integer,
    level integer,
    createdon timestamp without time zone
);


ALTER TABLE tfldata.stack_leaderboard OWNER TO tfldbrole;

--
-- TOC entry 242 (class 1259 OID 16866)
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
-- TOC entry 3724 (class 0 OID 0)
-- Dependencies: 242
-- Name: stack_leaderboard_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.stack_leaderboard_id_seq OWNED BY tfldata.stack_leaderboard.id;


--
-- TOC entry 227 (class 1259 OID 16640)
-- Name: users; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.users (
    id integer NOT NULL,
    username character varying(32) NOT NULL,
    password character varying(4096) NOT NULL,
    orgid character varying(256),
    pfp_name character varying(128),
    session_token uuid,
    allow_notification boolean,
    email character varying(64),
    firebase_user_uid character varying(64),
    fcm_registration_id character varying(168),
    gchat_bg_theme character varying(65),
    last_sign_on timestamp without time zone,
    gchat_order_option boolean,
    cf_domain_name character varying(30),
    is_admin boolean,
    last_pass_reset timestamp without time zone
);


ALTER TABLE tfldata.users OWNER TO tfldbrole;

--
-- TOC entry 228 (class 1259 OID 16645)
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
-- TOC entry 3725 (class 0 OID 0)
-- Dependencies: 228
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.users_id_seq OWNED BY tfldata.users.id;


--
-- TOC entry 244 (class 1259 OID 16893)
-- Name: users_to_threads; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.users_to_threads (
    username character varying(128),
    thread character varying(32),
    is_subscribed boolean
);


ALTER TABLE tfldata.users_to_threads OWNER TO tfldbrole;

--
-- TOC entry 3515 (class 2604 OID 16701)
-- Name: calendar id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.calendar ALTER COLUMN id SET DEFAULT nextval('tfldata.calendar_event_date_id_seq'::regclass);


--
-- TOC entry 3525 (class 2604 OID 16755)
-- Name: calendar_rsvp id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.calendar_rsvp ALTER COLUMN id SET DEFAULT nextval('tfldata.calendar_rsvp_id_seq'::regclass);


--
-- TOC entry 3516 (class 2604 OID 16702)
-- Name: comments id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.comments ALTER COLUMN id SET DEFAULT nextval('tfldata.comments_id_seq'::regclass);


--
-- TOC entry 3517 (class 2604 OID 16703)
-- Name: errlog id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.errlog ALTER COLUMN id SET DEFAULT nextval('tfldata.errlog_id_seq'::regclass);


--
-- TOC entry 3518 (class 2604 OID 16704)
-- Name: gchat id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.gchat ALTER COLUMN id SET DEFAULT nextval('tfldata.gchat_id_seq'::regclass);


--
-- TOC entry 3519 (class 2604 OID 16705)
-- Name: inclog id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.inclog ALTER COLUMN id SET DEFAULT nextval('tfldata.inclog_id_seq'::regclass);


--
-- TOC entry 3521 (class 2604 OID 16676)
-- Name: postfiles id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.postfiles ALTER COLUMN id SET DEFAULT nextval('tfldata.postfiles_id_seq'::regclass);


--
-- TOC entry 3522 (class 2604 OID 16721)
-- Name: posts id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.posts ALTER COLUMN id SET DEFAULT nextval('tfldata.posts_id_seq'::regclass);


--
-- TOC entry 3526 (class 2604 OID 16778)
-- Name: reactions id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.reactions ALTER COLUMN id SET DEFAULT nextval('tfldata.reactions_id_seq'::regclass);


--
-- TOC entry 3523 (class 2604 OID 16732)
-- Name: sent_notification_log id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.sent_notification_log ALTER COLUMN id SET DEFAULT nextval('tfldata.sent_notification_log_id_seq'::regclass);


--
-- TOC entry 3524 (class 2604 OID 16739)
-- Name: ss_leaderboard id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.ss_leaderboard ALTER COLUMN id SET DEFAULT nextval('tfldata.ss_leaderboard_id_seq'::regclass);


--
-- TOC entry 3527 (class 2604 OID 16870)
-- Name: stack_leaderboard id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.stack_leaderboard ALTER COLUMN id SET DEFAULT nextval('tfldata.stack_leaderboard_id_seq'::regclass);


--
-- TOC entry 3520 (class 2604 OID 16707)
-- Name: users id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.users ALTER COLUMN id SET DEFAULT nextval('tfldata.users_id_seq'::regclass);


--
-- TOC entry 3529 (class 2606 OID 16653)
-- Name: calendar calendar_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.calendar
    ADD CONSTRAINT calendar_pkey PRIMARY KEY (id);


--
-- TOC entry 3553 (class 2606 OID 16757)
-- Name: calendar_rsvp calendar_rsvp_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.calendar_rsvp
    ADD CONSTRAINT calendar_rsvp_pkey PRIMARY KEY (id);


--
-- TOC entry 3555 (class 2606 OID 16759)
-- Name: calendar_rsvp calendar_rsvp_username_event_id_key; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.calendar_rsvp
    ADD CONSTRAINT calendar_rsvp_username_event_id_key UNIQUE (username, event_id);


--
-- TOC entry 3531 (class 2606 OID 16655)
-- Name: comments comments_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.comments
    ADD CONSTRAINT comments_pkey PRIMARY KEY (id);


--
-- TOC entry 3533 (class 2606 OID 16657)
-- Name: errlog errlog_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.errlog
    ADD CONSTRAINT errlog_pkey PRIMARY KEY (id);


--
-- TOC entry 3535 (class 2606 OID 16659)
-- Name: gchat gchat_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.gchat
    ADD CONSTRAINT gchat_pkey PRIMARY KEY (id);


--
-- TOC entry 3537 (class 2606 OID 16661)
-- Name: inclog inclog_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.inclog
    ADD CONSTRAINT inclog_pkey PRIMARY KEY (id);


--
-- TOC entry 3545 (class 2606 OID 16678)
-- Name: postfiles postfiles_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.postfiles
    ADD CONSTRAINT postfiles_pkey PRIMARY KEY (id);


--
-- TOC entry 3547 (class 2606 OID 16725)
-- Name: posts posts_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.posts
    ADD CONSTRAINT posts_pkey PRIMARY KEY (id);


--
-- TOC entry 3557 (class 2606 OID 16784)
-- Name: reactions reactions_gchat_id_author_key; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.reactions
    ADD CONSTRAINT reactions_gchat_id_author_key UNIQUE (gchat_id, author);


--
-- TOC entry 3559 (class 2606 OID 16780)
-- Name: reactions reactions_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.reactions
    ADD CONSTRAINT reactions_pkey PRIMARY KEY (id);


--
-- TOC entry 3561 (class 2606 OID 16782)
-- Name: reactions reactions_post_id_author_key; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.reactions
    ADD CONSTRAINT reactions_post_id_author_key UNIQUE (post_id, author);


--
-- TOC entry 3549 (class 2606 OID 16734)
-- Name: sent_notification_log sent_notification_log_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.sent_notification_log
    ADD CONSTRAINT sent_notification_log_pkey PRIMARY KEY (id);


--
-- TOC entry 3539 (class 2606 OID 16663)
-- Name: sessions sessions_ip_addr_key; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.sessions
    ADD CONSTRAINT sessions_ip_addr_key UNIQUE (ip_addr);


--
-- TOC entry 3551 (class 2606 OID 16741)
-- Name: ss_leaderboard ss_leaderboard_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.ss_leaderboard
    ADD CONSTRAINT ss_leaderboard_pkey PRIMARY KEY (id);


--
-- TOC entry 3563 (class 2606 OID 16872)
-- Name: stack_leaderboard stack_leaderboard_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.stack_leaderboard
    ADD CONSTRAINT stack_leaderboard_pkey PRIMARY KEY (id);


--
-- TOC entry 3541 (class 2606 OID 16727)
-- Name: users users_email_key; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- TOC entry 3543 (class 2606 OID 16665)
-- Name: users users_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- TOC entry 3565 (class 2606 OID 16901)
-- Name: users_to_threads users_to_threads_username_thread_key; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.users_to_threads
    ADD CONSTRAINT users_to_threads_username_thread_key UNIQUE (username, thread);


-- Completed on 2023-12-07 04:46:24 EST

--
-- PostgreSQL database dump complete
--

