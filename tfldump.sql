--
-- PostgreSQL database dump
--

-- Dumped from database version 15.4
-- Dumped by pg_dump version 15.4

-- Started on 2023-09-21 15:25:24 EDT

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
-- TOC entry 6 (class 2615 OID 16401)
-- Name: tfldata; Type: SCHEMA; Schema: -; Owner: tfldbrole
--

CREATE SCHEMA tfldata;


ALTER SCHEMA tfldata OWNER TO tfldbrole;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- TOC entry 216 (class 1259 OID 16432)
-- Name: posts; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.posts (
    id integer NOT NULL,
    title character varying(128),
    description character varying(420),
    image_name character varying(64)
);


ALTER TABLE tfldata.posts OWNER TO tfldbrole;

--
-- TOC entry 215 (class 1259 OID 16431)
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
-- TOC entry 3592 (class 0 OID 0)
-- Dependencies: 215
-- Name: posts_id_seq; Type: SEQUENCE OWNED BY; Schema: tfldata; Owner: tfldbrole
--

ALTER SEQUENCE tfldata.posts_id_seq OWNED BY tfldata.posts.id;


--
-- TOC entry 3440 (class 2604 OID 16435)
-- Name: posts id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.posts ALTER COLUMN id SET DEFAULT nextval('tfldata.posts_id_seq'::regclass);


--
-- TOC entry 3586 (class 0 OID 16432)
-- Dependencies: 216
-- Data for Name: posts; Type: TABLE DATA; Schema: tfldata; Owner: tfldbrole
--

COPY tfldata.posts (id, title, description, image_name) FROM stdin;
48	cozy	Cuddles with daddy :)	1000000465.jpg
49	space baby	space baby is loving space!	1000000454.jpg
50	Picnic baby	Waiting for our boat to pick us up	1000000397.jpg
53	baby meets baby	jacob	IMG_1181.jpeg
52	crocodile tears	Awwww. He's faking	IMG_1110_Original.jpeg
55	Screaming	He aint happy...	1000000469.jpg
56	babyhood is exhausting 	and being milk drunk 🥛	IMG_4744.jpeg
\.


--
-- TOC entry 3593 (class 0 OID 0)
-- Dependencies: 215
-- Name: posts_id_seq; Type: SEQUENCE SET; Schema: tfldata; Owner: tfldbrole
--

SELECT pg_catalog.setval('tfldata.posts_id_seq', 56, true);


--
-- TOC entry 3442 (class 2606 OID 16439)
-- Name: posts posts_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.posts
    ADD CONSTRAINT posts_pkey PRIMARY KEY (id);


-- Completed on 2023-09-21 15:25:24 EDT

--
-- PostgreSQL database dump complete
--

