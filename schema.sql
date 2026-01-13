-- PostgreSQL database initialization script
-- Smart Home Backend Database Schema

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

-- Schema is created by default in Postgres


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- TOC entry 219 (class 1259 OID 16385)
-- Name: device_states_history; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.device_states_history (
    id integer NOT NULL,
    device_id text NOT NULL,
    "timestamp" timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    state jsonb,
    rule_id integer NOT NULL
);


ALTER TABLE public.device_states_history OWNER TO postgres;

--
-- TOC entry 220 (class 1259 OID 16392)
-- Name: device_states_history_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.device_states_history_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.device_states_history_id_seq OWNER TO postgres;

--
-- TOC entry 3482 (class 0 OID 0)
-- Dependencies: 220
-- Name: device_states_history_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.device_states_history_id_seq OWNED BY public.device_states_history.id;


--
-- TOC entry 221 (class 1259 OID 16393)
-- Name: devices; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.devices (
    name text NOT NULL,
    type text NOT NULL,
    state jsonb,
    mqtt_topic text NOT NULL,
    owner_id integer,
    accepted boolean DEFAULT false NOT NULL,
    id text CONSTRAINT devices_device_id_not_null NOT NULL
);


ALTER TABLE public.devices OWNER TO postgres;

--
-- TOC entry 222 (class 1259 OID 16403)
-- Name: rules; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.rules (
    name text NOT NULL,
    conditions jsonb NOT NULL,
    actions jsonb NOT NULL,
    enabled boolean DEFAULT true,
    owner_id integer,
    id integer NOT NULL
);


ALTER TABLE public.rules OWNER TO postgres;

--
-- TOC entry 223 (class 1259 OID 16413)
-- Name: rules_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

ALTER TABLE public.rules ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.rules_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- TOC entry 224 (class 1259 OID 16414)
-- Name: schedules; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.schedules (
    cron_expression text NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    id integer NOT NULL,
    rule_id integer NOT NULL
);


ALTER TABLE public.schedules OWNER TO postgres;

--
-- TOC entry 227 (class 1259 OID 32803)
-- Name: schedules_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

ALTER TABLE public.schedules ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.schedules_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- TOC entry 225 (class 1259 OID 16422)
-- Name: users; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.users (
    id integer NOT NULL,
    username text NOT NULL,
    password text NOT NULL,
    email text NOT NULL
);


ALTER TABLE public.users OWNER TO postgres;

--
-- TOC entry 226 (class 1259 OID 16431)
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

ALTER TABLE public.users ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- TOC entry 3313 (class 2606 OID 32785)
-- Name: device_states_history device_states_history_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.device_states_history
    ADD CONSTRAINT device_states_history_pkey PRIMARY KEY (id);


--
-- TOC entry 3315 (class 2606 OID 24596)
-- Name: devices devices_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.devices
    ADD CONSTRAINT devices_pkey PRIMARY KEY (id);


--
-- TOC entry 3319 (class 2606 OID 32794)
-- Name: rules rules_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rules
    ADD CONSTRAINT rules_pkey PRIMARY KEY (id);


--
-- TOC entry 3321 (class 2606 OID 32812)
-- Name: schedules schedules_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.schedules
    ADD CONSTRAINT schedules_pkey PRIMARY KEY (id);


--
-- TOC entry 3317 (class 2606 OID 24586)
-- Name: devices unique_device_id; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.devices
    ADD CONSTRAINT unique_device_id UNIQUE (id);


--
-- TOC entry 3323 (class 2606 OID 32770)
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- TOC entry 3324 (class 2606 OID 32788)
-- Name: device_states_history device_states_history_device_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.device_states_history
    ADD CONSTRAINT device_states_history_device_id_fkey FOREIGN KEY (device_id) REFERENCES public.devices(id) NOT VALID;


--
-- TOC entry 3325 (class 2606 OID 32795)
-- Name: device_states_history device_states_history_rule_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.device_states_history
    ADD CONSTRAINT device_states_history_rule_id_fkey FOREIGN KEY (rule_id) REFERENCES public.rules(id) NOT VALID;


--
-- TOC entry 3326 (class 2606 OID 32771)
-- Name: devices devices_owner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.devices
    ADD CONSTRAINT devices_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES public.users(id) NOT VALID;


--
-- TOC entry 3327 (class 2606 OID 32776)
-- Name: rules rules_owner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rules
    ADD CONSTRAINT rules_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES public.users(id) NOT VALID;


--
-- TOC entry 3328 (class 2606 OID 32813)
-- Name: schedules schedules_rule_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.schedules
    ADD CONSTRAINT schedules_rule_id_fkey FOREIGN KEY (rule_id) REFERENCES public.rules(id) NOT VALID;

