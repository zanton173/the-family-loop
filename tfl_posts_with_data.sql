--
-- PostgreSQL database dump
--

-- Dumped from database version 15.0
-- Dumped by pg_dump version 15.0

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

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: posts; Type: TABLE; Schema: tfldata; Owner: tfldbrole
--

CREATE TABLE tfldata.posts (
    id integer NOT NULL,
    title character varying(128),
    description character varying(420),
    author character varying(32),
    post_files_key uuid,
    createdon timestamp without time zone
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
-- Name: posts id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.posts ALTER COLUMN id SET DEFAULT nextval('tfldata.posts_id_seq'::regclass);


--
-- Data for Name: posts; Type: TABLE DATA; Schema: tfldata; Owner: tfldbrole
--

COPY tfldata.posts (id, title, description, author, post_files_key, createdon) FROM stdin;
192	Jacobs hair transformation	swipe to see it’s so crazy I’m cracking up	lillybelleanton	1947080b-e697-4e8b-8f5a-73942526d438	2023-01-01 22:20:05.078459
195	Jacobs first homemade cinnamon roll!	jk	lillybelleanton	630761b7-f8c7-45f1-ba13-d7220e2371bb	2023-01-02 22:20:24.364398
196	Its that time of year	Spicy	zanton	f358699a-f041-4e4c-8e9f-a050ee5a562a	2023-01-03 22:20:30.292299
197	POV when Jacob eats your nose	 	lillybelleanton	56652021-aa9f-43d2-8b26-eab918c3939a	2023-01-05 22:20:33.546519
198	Jacobs new clothes 😭	 	lillybelleanton	70157adb-2530-41b2-bf7e-dd484147abdb	2023-01-06 22:20:36.99876
199	BEFORE FIRST HAIRCUT	AHHHH	lillybelleanton	b7eeb695-1d56-46f6-8075-c9fbf3372e77	2023-01-08 22:20:41.320324
200	AFTER	 	lillybelleanton	7acd6274-cc56-4b4b-8499-4af75d8e189d	2023-01-10 22:20:45.02233
201	another pic of new haircut 	 	lillybelleanton	7164aec8-f597-49e8-a7b3-1041554326b8	2023-01-11 22:20:48.412452
202	which sides better 	 	lillybelleanton	aed7c5d7-a8bd-455b-9f9e-69827b6588b0	2023-01-12 22:20:54.768379
203	Jacobs obsessed with mugs atm	 	lillybelleanton	91a088f5-dc54-436d-9e07-d397d055b949	2023-01-13 22:20:58.671392
97	Solar System	Uncle Pete would be proud	Lilly	788cd02c-c4d8-45a0-b794-7f7da13f12d9	2022-12-07 22:14:11.355552
143	Gimme some space	Bottom Text	Pete	634c4da5-5635-456f-a402-b24b892a048e	2022-12-08 22:15:15.580496
146	Pumpkin Baby	worst day of my life	lillybelleanton	a763c137-a285-45a9-bec0-9472d400dc1a	2022-12-09 22:15:21.118821
148	Zach running in the 5K	He placed 3rd 	Mommy_grammy	a924195d-edde-461d-8a69-d614b7017d90	2022-12-10 22:15:26.862875
149	Pumpkin Baby!	I’m happy again!	lillybelleanton	25a03201-a5dd-44a6-9d32-0666d29e9161	2022-12-11 22:16:01.810442
150	Pumpkin Baby!	ugh so cute	lillybelleanton	6e63af26-ed9c-48a3-ba92-3ddcbdf71b7a	2022-12-12 22:16:07.99546
151	Stinky butt	uh oh stinky	lillybelleanton	44b27254-b078-484c-bca0-edb83e025c68	2022-12-13 22:16:16.310729
152	Hair time with daddy!	no wonder why people keep asking if I’m a girl	lillybelleanton	02b49530-1d00-46c6-8f85-80c22c2295ef	2022-12-14 22:16:21.504159
153	Revenge	 	lillybelleanton	5bcefb46-9164-4fab-bd51-3dbe47170501	2022-12-15 22:16:36.714823
154	why would you say that to me	>:(	annabelanton	9239952c-d4d4-4389-95f5-b9c998d4caf1	2022-12-16 22:16:41.152587
157	I’m a girl now or 2	literally stop	lillybelleanton	f90b0dd6-f4f2-43fa-a1f9-bfbcdd1953c4	2022-12-18 22:18:12.366375
158	Happy 4 Months!!	From this mornings photoshoot :)	lillybelleanton	15e490bf-c7dd-4e17-9bcc-d2229e03bc56	2022-12-19 22:18:27.548371
204	 	don’t worry I gave him a baby safe mug to make him happy	lillybelleanton	081ed80b-bce5-418f-af63-278202946471	2023-01-14 22:21:01.679059
176	Blue screen 	Zach can you help	pete	e5fbe397-04d0-4d75-99a7-76d17a927a43	2022-12-20 22:18:37.778267
177	Zach 	I don’t know 	mommy-grammy	c3ec8c19-e77c-4d14-8755-ce68cbcd27bf	2022-12-21 22:18:43.134159
178	Mornings with baby 😍	 	lillybelleanton	aedcd921-bb40-48a9-902d-e5e76ea9075d	2022-12-22 22:18:47.528172
179	Jacobs new favorite face	 	lillybelleanton	1166c905-8ad8-4c24-9219-a304bb95866c	2022-12-23 22:18:52.778238
184	LOLOL	this new face is CRACKING ME UP	lillybelleanton	fe8af3ad-f679-4d90-8010-c94b122eb486	2022-12-25 22:19:08.674526
185	Baby Jacob so cute 	Socute	pete	28ce85ee-5fd5-4527-970a-0668b3b94ca3	2022-12-26 22:19:33.305988
186	Squash this boy	Abby and K.C.	pop_pop	b976b64d-651e-456c-82d3-ca9d2e724f87	2022-12-27 22:19:39.615905
188	Poppop gave my baby a knife	 	lillybelleanton	b43dc0cd-5d06-45e9-90af-893525d04c56	2022-12-28 22:19:44.167385
205	Merry Christmas from the ChrEasters!	Do we need more ornaments, or is it good? Nicole says we're good I say add more	arta189	aa827f12-6b0e-4280-8a5e-80792a5702b3	2023-01-15 22:21:07.056318
189	JACOBS FIRST TOOTH	 	lillybelleanton	508f61e1-6c94-4e17-b359-663afb4f9815	2022-12-29 22:19:47.568235
190	Uh oh!	Annabel made a mess	pete	d377eebe-3f0a-495c-b90c-e5d8f59d0495	2022-12-30 22:19:53.948341
191	Jacob aging simulation	Ai is so crazy 😧	pete	eb85e642-6a1e-439c-9f59-927fdd4c4ac7	2022-12-31 22:20:00.452192
206	Jacobs first time in his high chair 😭	he’s literally so big I can’t handle it anymore 	lillybelleanton	51fb7468-be61-41f3-8dc2-9270dd9d18c8	2023-01-16 22:21:10.568383
208	Our tree rn bc half our lights are broken lol	 	lillybelleanton	415e7a89-3ea9-4d1d-a0f5-fe0bc143c178	2023-01-17 22:21:16.594399
209	Okay here’s our Christmas tree :)	 	lillybelleanton	ae769224-465f-4106-a5ab-bdfdc771a576	2023-01-18 22:21:27.028414
211	Christmas tree	It’s lit!	mommy-grammy	fb36d5c7-e577-4938-9826-0a15a5250cdc	2023-01-20 22:21:29.830378
212	Little	When Jacob was so small 	abbyyant	483b42a1-72ff-48e0-9152-71b4029de866	2023-01-21 22:21:38.448271
213	New hat #1	 	lillybelleanton	ca30402a-11cf-4637-968f-7f2e439d4815	2023-01-22 22:21:42.636185
214	New hat #2	my grandma made this 	lillybelleanton	1302b591-823b-4d04-a078-60d715670d3b	2023-12-05 22:22:08.868329
\.


--
-- Name: posts_id_seq; Type: SEQUENCE SET; Schema: tfldata; Owner: tfldbrole
--

SELECT pg_catalog.setval('tfldata.posts_id_seq', 214, true);


--
-- Name: posts posts_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.posts
    ADD CONSTRAINT posts_pkey PRIMARY KEY (id);


--
-- PostgreSQL database dump complete
--

