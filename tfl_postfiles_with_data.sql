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
-- Name: postfiles id; Type: DEFAULT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.postfiles ALTER COLUMN id SET DEFAULT nextval('tfldata.postfiles_id_seq'::regclass);


--
-- Data for Name: postfiles; Type: TABLE DATA; Schema: tfldata; Owner: tfldbrole
--

COPY tfldata.postfiles (id, file_name, file_type, post_files_key) FROM stdin;
1	IMG_5050.jpeg	image/jpeg	788cd02c-c4d8-45a0-b794-7f7da13f12d9
2	1000002076.png	image/png	634c4da5-5635-456f-a402-b24b892a048e
3	IMG_5414.jpeg	image/jpeg	02b49530-1d00-46c6-8f85-80c22c2295ef
4	IMG_5410.jpeg	image/jpeg	44b27254-b078-484c-bca0-edb83e025c68
5	IMG_3707.jpeg	image/jpeg	6e63af26-ed9c-48a3-ba92-3ddcbdf71b7a
6	IMG_3706.jpeg	image/jpeg	25a03201-a5dd-44a6-9d32-0666d29e9161
7	IMG_5524.mov	application/octet-stream	a924195d-edde-461d-8a69-d614b7017d90
8	IMG_3622.jpeg	image/jpeg	a763c137-a285-45a9-bec0-9472d400dc1a
9	IMG_5435.jpeg	image/jpeg	5bcefb46-9164-4fab-bd51-3dbe47170501
10	IMG_2926.jpeg	image/jpeg	9239952c-d4d4-4389-95f5-b9c998d4caf1
17	IMG_5423.jpeg	image/jpeg	f90b0dd6-f4f2-43fa-a1f9-bfbcdd1953c4
18	IMG_5428.jpeg	image/jpeg	f90b0dd6-f4f2-43fa-a1f9-bfbcdd1953c4
19	IMG_5492.jpeg	image/jpeg	15e490bf-c7dd-4e17-9bcc-d2229e03bc56
20	IMG_5482.jpeg	image/jpeg	15e490bf-c7dd-4e17-9bcc-d2229e03bc56
21	IMG_5532.jpeg	image/jpeg	15e490bf-c7dd-4e17-9bcc-d2229e03bc56
22	IMG_5474.jpeg	image/jpeg	15e490bf-c7dd-4e17-9bcc-d2229e03bc56
23	IMG_5542.jpeg	image/jpeg	15e490bf-c7dd-4e17-9bcc-d2229e03bc56
24	IMG_5556.jpeg	image/jpeg	15e490bf-c7dd-4e17-9bcc-d2229e03bc56
25	IMG_5525.jpeg	image/jpeg	15e490bf-c7dd-4e17-9bcc-d2229e03bc56
28	1000002317.jpg	image/jpeg	e5fbe397-04d0-4d75-99a7-76d17a927a43
29	1000002318.jpg	image/jpeg	e5fbe397-04d0-4d75-99a7-76d17a927a43
30	IMG_5801.jpeg	image/jpeg	c3ec8c19-e77c-4d14-8755-ce68cbcd27bf
31	IMG_5748.jpeg	image/jpeg	aedcd921-bb40-48a9-902d-e5e76ea9075d
32	IMG_5747.jpeg	image/jpeg	aedcd921-bb40-48a9-902d-e5e76ea9075d
33	IMG_5746.jpeg	image/jpeg	aedcd921-bb40-48a9-902d-e5e76ea9075d
34	IMG_5745.jpeg	image/jpeg	aedcd921-bb40-48a9-902d-e5e76ea9075d
35	IMG_5800.jpeg	image/jpeg	1166c905-8ad8-4c24-9219-a304bb95866c
36	IMG_5799.jpeg	image/jpeg	1166c905-8ad8-4c24-9219-a304bb95866c
37	IMG_5806.png	image/jpeg	1166c905-8ad8-4c24-9219-a304bb95866c
42	focused_jacob1.png	image/png	28ce85ee-5fd5-4527-970a-0668b3b94ca3
43	footsies.png	image/png	28ce85ee-5fd5-4527-970a-0668b3b94ca3
44	jacob_abby.png	image/png	28ce85ee-5fd5-4527-970a-0668b3b94ca3
45	jacob_looking1.png	image/png	28ce85ee-5fd5-4527-970a-0668b3b94ca3
46	IMG_5828.mov	application/octet-stream	fe8af3ad-f679-4d90-8010-c94b122eb486
47	concerned_jacob1.png	image/png	28ce85ee-5fd5-4527-970a-0668b3b94ca3
48	funny_jacob.png	image/png	28ce85ee-5fd5-4527-970a-0668b3b94ca3
49	jacob_thumb.png	image/png	28ce85ee-5fd5-4527-970a-0668b3b94ca3
50	sleepy_jacob.png	image/png	28ce85ee-5fd5-4527-970a-0668b3b94ca3
51	jacob_belly.png	image/png	28ce85ee-5fd5-4527-970a-0668b3b94ca3
52	20230930_181530.mp4	video/mp4	b976b64d-651e-456c-82d3-ca9d2e724f87
54	IMG_5933.jpeg	image/jpeg	b43dc0cd-5d06-45e9-90af-893525d04c56
55	IMG_5931.jpeg	image/jpeg	b43dc0cd-5d06-45e9-90af-893525d04c56
56	IMG_6079.mov	application/octet-stream	508f61e1-6c94-4e17-b359-663afb4f9815
57	1000002583.jpg	image/jpeg	d377eebe-3f0a-495c-b90c-e5d8f59d0495
58	1000002607.mp4	video/mp4	eb85e642-6a1e-439c-9f59-927fdd4c4ac7
59	IMG_6261.png	image/png	1947080b-e697-4e8b-8f5a-73942526d438
60	IMG_6260.png	image/png	1947080b-e697-4e8b-8f5a-73942526d438
61	IMG_6259.png	image/png	1947080b-e697-4e8b-8f5a-73942526d438
62	IMG_6258.png	image/png	1947080b-e697-4e8b-8f5a-73942526d438
63	IMG_6257.png	image/png	1947080b-e697-4e8b-8f5a-73942526d438
66	IMG_6309.jpeg	image/jpeg	630761b7-f8c7-45f1-ba13-d7220e2371bb
67	IMG_6305.jpeg	image/jpeg	630761b7-f8c7-45f1-ba13-d7220e2371bb
68	20231121_084042.jpg	image/jpeg	f358699a-f041-4e4c-8e9f-a050ee5a562a
69	IMG_6332.mov	application/octet-stream	56652021-aa9f-43d2-8b26-eab918c3939a
70	IMG_6545.jpeg	image/jpeg	70157adb-2530-41b2-bf7e-dd484147abdb
71	IMG_6546.jpeg	image/jpeg	70157adb-2530-41b2-bf7e-dd484147abdb
72	IMG_6514.jpeg	image/jpeg	70157adb-2530-41b2-bf7e-dd484147abdb
73	IMG_6513.jpeg	image/jpeg	70157adb-2530-41b2-bf7e-dd484147abdb
74	IMG_6562.jpeg	image/jpeg	b7eeb695-1d56-46f6-8075-c9fbf3372e77
75	IMG_6565.jpeg	image/jpeg	b7eeb695-1d56-46f6-8075-c9fbf3372e77
76	IMG_6597.jpeg	image/jpeg	7acd6274-cc56-4b4b-8499-4af75d8e189d
77	IMG_6592.jpeg	image/jpeg	7acd6274-cc56-4b4b-8499-4af75d8e189d
78	IMG_6570.jpeg	image/jpeg	7acd6274-cc56-4b4b-8499-4af75d8e189d
79	IMG_6573.jpeg	image/jpeg	7acd6274-cc56-4b4b-8499-4af75d8e189d
80	IMG_6600.jpeg	image/jpeg	7164aec8-f597-49e8-a7b3-1041554326b8
81	IMG_6605.jpeg	image/jpeg	aed7c5d7-a8bd-455b-9f9e-69827b6588b0
82	IMG_6603.jpeg	image/jpeg	aed7c5d7-a8bd-455b-9f9e-69827b6588b0
83	IMG_6659.mov	application/octet-stream	91a088f5-dc54-436d-9e07-d397d055b949
84	IMG_6657.jpeg	image/jpeg	081ed80b-bce5-418f-af63-278202946471
85	IMG_6655.jpeg	image/jpeg	081ed80b-bce5-418f-af63-278202946471
86	20231203_160642.jpg	image/jpeg	aa827f12-6b0e-4280-8a5e-80792a5702b3
87	IMG_6697.jpeg	image/jpeg	51fb7468-be61-41f3-8dc2-9270dd9d18c8
89	72342610596__53BB5148-DBBE-448D-8581-A65489BB24E3.jpeg	image/jpeg	415e7a89-3ea9-4d1d-a0f5-fe0bc143c178
90	IMG_6721.jpeg	image/jpeg	ae769224-465f-4106-a5ab-bdfdc771a576
92	image.jpg	image/jpeg	fb36d5c7-e577-4938-9826-0a15a5250cdc
93	IMG_5269.jpeg	image/jpeg	483b42a1-72ff-48e0-9152-71b4029de866
94	IMG_6741.jpeg	image/jpeg	ca30402a-11cf-4637-968f-7f2e439d4815
95	IMG_6743.jpeg	image/jpeg	ca30402a-11cf-4637-968f-7f2e439d4815
96	IMG_6746.jpeg	image/jpeg	1302b591-823b-4d04-a078-60d715670d3b
\.


--
-- Name: postfiles_id_seq; Type: SEQUENCE SET; Schema: tfldata; Owner: tfldbrole
--

SELECT pg_catalog.setval('tfldata.postfiles_id_seq', 96, true);


--
-- Name: postfiles postfiles_pkey; Type: CONSTRAINT; Schema: tfldata; Owner: tfldbrole
--

ALTER TABLE ONLY tfldata.postfiles
    ADD CONSTRAINT postfiles_pkey PRIMARY KEY (id);


--
-- PostgreSQL database dump complete
--

