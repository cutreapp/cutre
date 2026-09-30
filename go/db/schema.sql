

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

--
-- Name: citext; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS citext WITH SCHEMA public;


--
-- Name: EXTENSION citext; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION citext IS 'data type for case-insensitive character strings';


--
-- Name: event_category_status; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.event_category_status AS ENUM (
    'published',
    'archived',
    'deleted'
);


--
-- Name: event_status; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.event_status AS ENUM (
    'published',
    'archived',
    'deleted'
);


--
-- Name: goods_status; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.goods_status AS ENUM (
    'published',
    'archived',
    'deleted'
);


--
-- Name: item_kind; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.item_kind AS ENUM (
    'give',
    'want'
);


--
-- Name: item_status; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.item_status AS ENUM (
    'listed',
    'removed'
);


--
-- Name: river_job_state; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.river_job_state AS ENUM (
    'available',
    'cancelled',
    'completed',
    'discarded',
    'pending',
    'retryable',
    'running',
    'scheduled'
);


--
-- Name: station_status; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.station_status AS ENUM (
    'published',
    'archived',
    'deleted'
);


--
-- Name: trade_event_kind; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.trade_event_kind AS ENUM (
    'proposed',
    'withdrawn',
    'approved',
    'declined',
    'completed',
    'failed',
    'cancelled'
);


--
-- Name: trade_status; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.trade_status AS ENUM (
    'pending',
    'withdrawn',
    'declined',
    'matched',
    'completed',
    'failed',
    'cancelled'
);


--
-- Name: user_role; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.user_role AS ENUM (
    'user',
    'editor',
    'admin'
);


--
-- Name: river_job_state_in_bitmask(bit, public.river_job_state); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.river_job_state_in_bitmask(bitmask bit, state public.river_job_state) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $$
    SELECT CASE state
        WHEN 'available' THEN get_bit(bitmask, 7)
        WHEN 'cancelled' THEN get_bit(bitmask, 6)
        WHEN 'completed' THEN get_bit(bitmask, 5)
        WHEN 'discarded' THEN get_bit(bitmask, 4)
        WHEN 'pending'   THEN get_bit(bitmask, 3)
        WHEN 'retryable' THEN get_bit(bitmask, 2)
        WHEN 'running'   THEN get_bit(bitmask, 1)
        WHEN 'scheduled' THEN get_bit(bitmask, 0)
        ELSE 0
    END = 1;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: email_confirmations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.email_confirmations (
    id uuid DEFAULT uuidv7() NOT NULL,
    email public.citext NOT NULL,
    code character varying NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    failed_attempts_count integer DEFAULT 0 NOT NULL,
    confirmed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: event_categories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.event_categories (
    id uuid DEFAULT uuidv7() NOT NULL,
    event_id uuid NOT NULL,
    name character varying NOT NULL,
    "position" integer NOT NULL,
    status public.event_category_status DEFAULT 'published'::public.event_category_status NOT NULL,
    archive_message character varying,
    lock_version integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT event_categories_position_check CHECK (("position" >= 0))
);


--
-- Name: events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.events (
    id uuid DEFAULT uuidv7() NOT NULL,
    name character varying NOT NULL,
    starts_on date NOT NULL,
    ends_on date,
    status public.event_status DEFAULT 'published'::public.event_status NOT NULL,
    archive_message character varying,
    lock_version integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT events_check CHECK (((ends_on IS NULL) OR (starts_on <= ends_on)))
);


--
-- Name: goods; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.goods (
    id uuid DEFAULT uuidv7() NOT NULL,
    event_category_id uuid NOT NULL,
    name character varying NOT NULL,
    "position" integer NOT NULL,
    status public.goods_status DEFAULT 'published'::public.goods_status NOT NULL,
    archive_message character varying,
    lock_version integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT goods_position_check CHECK (("position" >= 0))
);


--
-- Name: invitation_redemptions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.invitation_redemptions (
    id uuid DEFAULT uuidv7() NOT NULL,
    invitation_id uuid NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: invitations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.invitations (
    id uuid DEFAULT uuidv7() NOT NULL,
    inviter_user_id uuid,
    token character varying NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    revoked_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.items (
    id uuid DEFAULT uuidv7() NOT NULL,
    user_id uuid NOT NULL,
    goods_id uuid NOT NULL,
    kind public.item_kind NOT NULL,
    status public.item_status DEFAULT 'listed'::public.item_status NOT NULL,
    quantity integer NOT NULL,
    note character varying DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    lock_version integer DEFAULT 0 NOT NULL,
    CONSTRAINT items_quantity_check CHECK ((quantity >= 0))
);


--
-- Name: message_consents; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.message_consents (
    id uuid DEFAULT uuidv7() NOT NULL,
    user_id uuid NOT NULL,
    version integer NOT NULL,
    agreed_at timestamp with time zone NOT NULL,
    withdrawn_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: password_reset_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.password_reset_tokens (
    id uuid DEFAULT uuidv7() NOT NULL,
    user_id uuid NOT NULL,
    token_digest character varying NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: rate_limits; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rate_limits (
    id uuid DEFAULT uuidv7() NOT NULL,
    key character varying NOT NULL,
    window_start timestamp with time zone NOT NULL,
    count integer DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: river_job; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.river_job (
    id bigint NOT NULL,
    state public.river_job_state DEFAULT 'available'::public.river_job_state NOT NULL,
    attempt smallint DEFAULT 0 NOT NULL,
    max_attempts smallint DEFAULT 25 NOT NULL,
    attempted_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    finalized_at timestamp with time zone,
    scheduled_at timestamp with time zone DEFAULT now() NOT NULL,
    priority smallint DEFAULT 1 NOT NULL,
    args jsonb NOT NULL,
    attempted_by text[],
    errors jsonb[],
    kind text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    queue text DEFAULT 'default'::text NOT NULL,
    tags character varying(255)[] DEFAULT '{}'::character varying[] NOT NULL,
    unique_key bytea,
    unique_states bit(8),
    CONSTRAINT finalized_or_finalized_at_null CHECK ((((finalized_at IS NULL) AND (state <> ALL (ARRAY['cancelled'::public.river_job_state, 'completed'::public.river_job_state, 'discarded'::public.river_job_state]))) OR ((finalized_at IS NOT NULL) AND (state = ANY (ARRAY['cancelled'::public.river_job_state, 'completed'::public.river_job_state, 'discarded'::public.river_job_state]))))),
    CONSTRAINT kind_length CHECK (((char_length(kind) > 0) AND (char_length(kind) < 128))),
    CONSTRAINT max_attempts_is_positive CHECK ((max_attempts > 0)),
    CONSTRAINT priority_in_range CHECK (((priority >= 1) AND (priority <= 4))),
    CONSTRAINT queue_length CHECK (((char_length(queue) > 0) AND (char_length(queue) < 128)))
);


--
-- Name: river_job_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.river_job_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: river_job_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.river_job_id_seq OWNED BY public.river_job.id;


--
-- Name: river_leader; Type: TABLE; Schema: public; Owner: -
--

CREATE UNLOGGED TABLE public.river_leader (
    elected_at timestamp with time zone NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    leader_id text NOT NULL,
    name text DEFAULT 'default'::text NOT NULL,
    CONSTRAINT leader_id_length CHECK (((char_length(leader_id) > 0) AND (char_length(leader_id) < 128))),
    CONSTRAINT name_length CHECK ((name = 'default'::text))
);


--
-- Name: river_migration; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.river_migration (
    line text NOT NULL,
    version bigint CONSTRAINT river_migration_version_not_null1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT river_migration_created_at_not_null1 NOT NULL,
    CONSTRAINT line_length CHECK (((char_length(line) > 0) AND (char_length(line) < 128))),
    CONSTRAINT version_gte_1 CHECK ((version >= 1))
);


--
-- Name: river_notification; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.river_notification (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    payload text NOT NULL,
    topic text NOT NULL,
    CONSTRAINT topic_length CHECK (((length(topic) > 0) AND (length(topic) < 128)))
);


--
-- Name: river_notification_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.river_notification_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: river_notification_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.river_notification_id_seq OWNED BY public.river_notification.id;


--
-- Name: river_queue; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.river_queue (
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    paused_at timestamp with time zone,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version character varying NOT NULL
);


--
-- Name: stations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.stations (
    id uuid DEFAULT uuidv7() NOT NULL,
    prefecture_code smallint NOT NULL,
    name character varying NOT NULL,
    "position" integer NOT NULL,
    status public.station_status DEFAULT 'published'::public.station_status NOT NULL,
    archive_message character varying,
    lock_version integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT stations_position_check CHECK (("position" >= 0)),
    CONSTRAINT stations_prefecture_code_check CHECK (((prefecture_code >= 1) AND (prefecture_code <= 47)))
);


--
-- Name: trade_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.trade_events (
    id uuid DEFAULT uuidv7() NOT NULL,
    trade_id uuid NOT NULL,
    actor_user_id uuid NOT NULL,
    kind public.trade_event_kind NOT NULL,
    reason character varying,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: trade_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.trade_items (
    id uuid DEFAULT uuidv7() NOT NULL,
    trade_id uuid NOT NULL,
    item_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: trade_message_reads; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.trade_message_reads (
    id uuid DEFAULT uuidv7() NOT NULL,
    trade_id uuid NOT NULL,
    user_id uuid NOT NULL,
    last_read_at timestamp with time zone NOT NULL,
    last_read_message_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: trade_messages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.trade_messages (
    id uuid DEFAULT uuidv7() NOT NULL,
    trade_id uuid NOT NULL,
    sender_user_id uuid NOT NULL,
    body character varying NOT NULL,
    retracted_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: trades; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.trades (
    id uuid DEFAULT uuidv7() NOT NULL,
    proposer_user_id uuid NOT NULL,
    receiver_user_id uuid NOT NULL,
    status public.trade_status DEFAULT 'pending'::public.trade_status NOT NULL,
    proposer_completed_at timestamp with time zone,
    receiver_completed_at timestamp with time zone,
    matched_at timestamp with time zone,
    ended_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT trades_check CHECK ((proposer_user_id <> receiver_user_id))
);


--
-- Name: user_passwords; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_passwords (
    id uuid DEFAULT uuidv7() NOT NULL,
    user_id uuid NOT NULL,
    password_digest character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_sessions (
    id uuid DEFAULT uuidv7() NOT NULL,
    user_id uuid NOT NULL,
    token_digest character varying NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL,
    ip_address character varying NOT NULL,
    user_agent character varying NOT NULL,
    signed_in_at timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_stations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_stations (
    id uuid DEFAULT uuidv7() NOT NULL,
    user_id uuid NOT NULL,
    station_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_two_factor_auths; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_two_factor_auths (
    id uuid DEFAULT uuidv7() NOT NULL,
    user_id uuid NOT NULL,
    secret_ciphertext bytea NOT NULL,
    last_used_step bigint DEFAULT 0 NOT NULL,
    enabled_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_two_factor_recovery_codes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_two_factor_recovery_codes (
    id uuid DEFAULT uuidv7() NOT NULL,
    user_id uuid NOT NULL,
    code_digest character varying NOT NULL,
    used_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id uuid DEFAULT uuidv7() NOT NULL,
    email public.citext NOT NULL,
    atname public.citext NOT NULL,
    locale character varying NOT NULL,
    time_zone character varying NOT NULL,
    deleted_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    role public.user_role DEFAULT 'user'::public.user_role NOT NULL,
    place_note character varying DEFAULT ''::character varying NOT NULL,
    place_lock_version integer DEFAULT 0 NOT NULL
);


--
-- Name: river_job id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_job ALTER COLUMN id SET DEFAULT nextval('public.river_job_id_seq'::regclass);


--
-- Name: river_notification id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_notification ALTER COLUMN id SET DEFAULT nextval('public.river_notification_id_seq'::regclass);


--
-- Name: email_confirmations email_confirmations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.email_confirmations
    ADD CONSTRAINT email_confirmations_pkey PRIMARY KEY (id);


--
-- Name: event_categories event_categories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.event_categories
    ADD CONSTRAINT event_categories_pkey PRIMARY KEY (id);


--
-- Name: events events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.events
    ADD CONSTRAINT events_pkey PRIMARY KEY (id);


--
-- Name: goods goods_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goods
    ADD CONSTRAINT goods_pkey PRIMARY KEY (id);


--
-- Name: invitation_redemptions invitation_redemptions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invitation_redemptions
    ADD CONSTRAINT invitation_redemptions_pkey PRIMARY KEY (id);


--
-- Name: invitation_redemptions invitation_redemptions_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invitation_redemptions
    ADD CONSTRAINT invitation_redemptions_user_id_key UNIQUE (user_id);


--
-- Name: invitations invitations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invitations
    ADD CONSTRAINT invitations_pkey PRIMARY KEY (id);


--
-- Name: invitations invitations_token_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invitations
    ADD CONSTRAINT invitations_token_key UNIQUE (token);


--
-- Name: items items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.items
    ADD CONSTRAINT items_pkey PRIMARY KEY (id);


--
-- Name: message_consents message_consents_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_consents
    ADD CONSTRAINT message_consents_pkey PRIMARY KEY (id);


--
-- Name: password_reset_tokens password_reset_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_pkey PRIMARY KEY (id);


--
-- Name: password_reset_tokens password_reset_tokens_token_digest_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_token_digest_key UNIQUE (token_digest);


--
-- Name: password_reset_tokens password_reset_tokens_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_user_id_key UNIQUE (user_id);


--
-- Name: rate_limits rate_limits_key_window_start_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rate_limits
    ADD CONSTRAINT rate_limits_key_window_start_key UNIQUE (key, window_start);


--
-- Name: rate_limits rate_limits_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rate_limits
    ADD CONSTRAINT rate_limits_pkey PRIMARY KEY (id);


--
-- Name: river_job river_job_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_job
    ADD CONSTRAINT river_job_pkey PRIMARY KEY (id);


--
-- Name: river_leader river_leader_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_leader
    ADD CONSTRAINT river_leader_pkey PRIMARY KEY (name);


--
-- Name: river_migration river_migration_pkey1; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_migration
    ADD CONSTRAINT river_migration_pkey1 PRIMARY KEY (line, version);


--
-- Name: river_notification river_notification_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_notification
    ADD CONSTRAINT river_notification_pkey PRIMARY KEY (id);


--
-- Name: river_queue river_queue_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_queue
    ADD CONSTRAINT river_queue_pkey PRIMARY KEY (name);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: stations stations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stations
    ADD CONSTRAINT stations_pkey PRIMARY KEY (id);


--
-- Name: trade_events trade_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trade_events
    ADD CONSTRAINT trade_events_pkey PRIMARY KEY (id);


--
-- Name: trade_items trade_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trade_items
    ADD CONSTRAINT trade_items_pkey PRIMARY KEY (id);


--
-- Name: trade_message_reads trade_message_reads_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trade_message_reads
    ADD CONSTRAINT trade_message_reads_pkey PRIMARY KEY (id);


--
-- Name: trade_messages trade_messages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trade_messages
    ADD CONSTRAINT trade_messages_pkey PRIMARY KEY (id);


--
-- Name: trades trades_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trades
    ADD CONSTRAINT trades_pkey PRIMARY KEY (id);


--
-- Name: user_passwords user_passwords_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_passwords
    ADD CONSTRAINT user_passwords_pkey PRIMARY KEY (id);


--
-- Name: user_passwords user_passwords_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_passwords
    ADD CONSTRAINT user_passwords_user_id_key UNIQUE (user_id);


--
-- Name: user_sessions user_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_sessions
    ADD CONSTRAINT user_sessions_pkey PRIMARY KEY (id);


--
-- Name: user_sessions user_sessions_token_digest_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_sessions
    ADD CONSTRAINT user_sessions_token_digest_key UNIQUE (token_digest);


--
-- Name: user_stations user_stations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_stations
    ADD CONSTRAINT user_stations_pkey PRIMARY KEY (id);


--
-- Name: user_stations user_stations_user_id_station_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_stations
    ADD CONSTRAINT user_stations_user_id_station_id_key UNIQUE (user_id, station_id);


--
-- Name: user_two_factor_auths user_two_factor_auths_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_two_factor_auths
    ADD CONSTRAINT user_two_factor_auths_pkey PRIMARY KEY (id);


--
-- Name: user_two_factor_auths user_two_factor_auths_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_two_factor_auths
    ADD CONSTRAINT user_two_factor_auths_user_id_key UNIQUE (user_id);


--
-- Name: user_two_factor_recovery_codes user_two_factor_recovery_codes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_two_factor_recovery_codes
    ADD CONSTRAINT user_two_factor_recovery_codes_pkey PRIMARY KEY (id);


--
-- Name: user_two_factor_recovery_codes user_two_factor_recovery_codes_user_id_code_digest_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_two_factor_recovery_codes
    ADD CONSTRAINT user_two_factor_recovery_codes_user_id_code_digest_key UNIQUE (user_id, code_digest);


--
-- Name: users users_atname_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_atname_key UNIQUE (atname);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: email_confirmations_email_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX email_confirmations_email_idx ON public.email_confirmations USING btree (email);


--
-- Name: email_confirmations_expires_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX email_confirmations_expires_at_idx ON public.email_confirmations USING btree (expires_at);


--
-- Name: event_categories_event_id_position_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX event_categories_event_id_position_idx ON public.event_categories USING btree (event_id, "position");


--
-- Name: event_categories_event_id_position_idx1; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX event_categories_event_id_position_idx1 ON public.event_categories USING btree (event_id, "position") WHERE (status = 'published'::public.event_category_status);


--
-- Name: events_starts_on_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX events_starts_on_idx ON public.events USING btree (starts_on DESC) WHERE (status = 'published'::public.event_status);


--
-- Name: goods_event_category_id_position_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX goods_event_category_id_position_idx ON public.goods USING btree (event_category_id, "position");


--
-- Name: goods_event_category_id_position_idx1; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX goods_event_category_id_position_idx1 ON public.goods USING btree (event_category_id, "position") WHERE (status = 'published'::public.goods_status);


--
-- Name: invitation_redemptions_invitation_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX invitation_redemptions_invitation_id_idx ON public.invitation_redemptions USING btree (invitation_id);


--
-- Name: invitations_inviter_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX invitations_inviter_user_id_idx ON public.invitations USING btree (inviter_user_id);


--
-- Name: invitations_inviter_user_id_unrevoked_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX invitations_inviter_user_id_unrevoked_key ON public.invitations USING btree (inviter_user_id) WHERE (revoked_at IS NULL);


--
-- Name: items_goods_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX items_goods_id_idx ON public.items USING btree (goods_id);


--
-- Name: items_goods_id_kind_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX items_goods_id_kind_idx ON public.items USING btree (goods_id, kind) WHERE (status = 'listed'::public.item_status);


--
-- Name: items_user_id_goods_id_kind_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX items_user_id_goods_id_kind_idx ON public.items USING btree (user_id, goods_id, kind) WHERE (status = 'listed'::public.item_status);


--
-- Name: message_consents_user_id_agreed_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX message_consents_user_id_agreed_at_idx ON public.message_consents USING btree (user_id, agreed_at);


--
-- Name: rate_limits_window_start_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX rate_limits_window_start_idx ON public.rate_limits USING btree (window_start);


--
-- Name: river_job_args_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_args_index ON public.river_job USING gin (args);


--
-- Name: river_job_kind; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_kind ON public.river_job USING btree (kind);


--
-- Name: river_job_metadata_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_metadata_index ON public.river_job USING gin (metadata);


--
-- Name: river_job_prioritized_fetching_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_prioritized_fetching_index ON public.river_job USING btree (state, queue, priority, scheduled_at, id);


--
-- Name: river_job_state_and_finalized_at_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_state_and_finalized_at_index ON public.river_job USING btree (state, finalized_at) WHERE (finalized_at IS NOT NULL);


--
-- Name: river_job_unique_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX river_job_unique_idx ON public.river_job USING btree (unique_key) WHERE ((unique_key IS NOT NULL) AND (unique_states IS NOT NULL) AND public.river_job_state_in_bitmask(unique_states, state));


--
-- Name: river_notification_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_notification_created_at_idx ON public.river_notification USING btree (created_at);


--
-- Name: river_notification_topic_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_notification_topic_id_idx ON public.river_notification USING btree (topic, id);


--
-- Name: stations_prefecture_code_position_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX stations_prefecture_code_position_idx ON public.stations USING btree (prefecture_code, "position");


--
-- Name: stations_prefecture_code_position_idx1; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX stations_prefecture_code_position_idx1 ON public.stations USING btree (prefecture_code, "position") WHERE (status = 'published'::public.station_status);


--
-- Name: trade_events_actor_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX trade_events_actor_user_id_idx ON public.trade_events USING btree (actor_user_id);


--
-- Name: trade_events_trade_id_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX trade_events_trade_id_created_at_idx ON public.trade_events USING btree (trade_id, created_at);


--
-- Name: trade_items_item_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX trade_items_item_id_idx ON public.trade_items USING btree (item_id);


--
-- Name: trade_items_trade_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX trade_items_trade_id_idx ON public.trade_items USING btree (trade_id);


--
-- Name: trade_message_reads_trade_id_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX trade_message_reads_trade_id_user_id_idx ON public.trade_message_reads USING btree (trade_id, user_id);


--
-- Name: trade_message_reads_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX trade_message_reads_user_id_idx ON public.trade_message_reads USING btree (user_id);


--
-- Name: trade_messages_sender_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX trade_messages_sender_user_id_idx ON public.trade_messages USING btree (sender_user_id);


--
-- Name: trade_messages_trade_id_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX trade_messages_trade_id_created_at_idx ON public.trade_messages USING btree (trade_id, created_at);


--
-- Name: trade_messages_trade_id_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX trade_messages_trade_id_id_idx ON public.trade_messages USING btree (trade_id, id);


--
-- Name: trades_proposer_user_id_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX trades_proposer_user_id_status_idx ON public.trades USING btree (proposer_user_id, status);


--
-- Name: trades_receiver_user_id_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX trades_receiver_user_id_status_idx ON public.trades USING btree (receiver_user_id, status);


--
-- Name: user_sessions_expires_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_sessions_expires_at_idx ON public.user_sessions USING btree (expires_at);


--
-- Name: user_sessions_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_sessions_user_id_idx ON public.user_sessions USING btree (user_id);


--
-- Name: user_stations_station_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_stations_station_id_idx ON public.user_stations USING btree (station_id);


--
-- Name: event_categories event_categories_event_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.event_categories
    ADD CONSTRAINT event_categories_event_id_fkey FOREIGN KEY (event_id) REFERENCES public.events(id) ON DELETE CASCADE;


--
-- Name: goods goods_event_category_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goods
    ADD CONSTRAINT goods_event_category_id_fkey FOREIGN KEY (event_category_id) REFERENCES public.event_categories(id) ON DELETE CASCADE;


--
-- Name: invitation_redemptions invitation_redemptions_invitation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invitation_redemptions
    ADD CONSTRAINT invitation_redemptions_invitation_id_fkey FOREIGN KEY (invitation_id) REFERENCES public.invitations(id) ON DELETE RESTRICT;


--
-- Name: invitation_redemptions invitation_redemptions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invitation_redemptions
    ADD CONSTRAINT invitation_redemptions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: invitations invitations_inviter_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invitations
    ADD CONSTRAINT invitations_inviter_user_id_fkey FOREIGN KEY (inviter_user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: items items_goods_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.items
    ADD CONSTRAINT items_goods_id_fkey FOREIGN KEY (goods_id) REFERENCES public.goods(id) ON DELETE RESTRICT;


--
-- Name: items items_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.items
    ADD CONSTRAINT items_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: message_consents message_consents_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.message_consents
    ADD CONSTRAINT message_consents_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: password_reset_tokens password_reset_tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: trade_events trade_events_actor_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trade_events
    ADD CONSTRAINT trade_events_actor_user_id_fkey FOREIGN KEY (actor_user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: trade_events trade_events_trade_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trade_events
    ADD CONSTRAINT trade_events_trade_id_fkey FOREIGN KEY (trade_id) REFERENCES public.trades(id) ON DELETE CASCADE;


--
-- Name: trade_items trade_items_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trade_items
    ADD CONSTRAINT trade_items_item_id_fkey FOREIGN KEY (item_id) REFERENCES public.items(id) ON DELETE RESTRICT;


--
-- Name: trade_items trade_items_trade_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trade_items
    ADD CONSTRAINT trade_items_trade_id_fkey FOREIGN KEY (trade_id) REFERENCES public.trades(id) ON DELETE CASCADE;


--
-- Name: trade_message_reads trade_message_reads_trade_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trade_message_reads
    ADD CONSTRAINT trade_message_reads_trade_id_fkey FOREIGN KEY (trade_id) REFERENCES public.trades(id) ON DELETE CASCADE;


--
-- Name: trade_message_reads trade_message_reads_trade_id_last_read_message_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trade_message_reads
    ADD CONSTRAINT trade_message_reads_trade_id_last_read_message_id_fkey FOREIGN KEY (trade_id, last_read_message_id) REFERENCES public.trade_messages(trade_id, id) ON DELETE CASCADE;


--
-- Name: trade_message_reads trade_message_reads_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trade_message_reads
    ADD CONSTRAINT trade_message_reads_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: trade_messages trade_messages_sender_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trade_messages
    ADD CONSTRAINT trade_messages_sender_user_id_fkey FOREIGN KEY (sender_user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: trade_messages trade_messages_trade_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trade_messages
    ADD CONSTRAINT trade_messages_trade_id_fkey FOREIGN KEY (trade_id) REFERENCES public.trades(id) ON DELETE CASCADE;


--
-- Name: trades trades_proposer_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trades
    ADD CONSTRAINT trades_proposer_user_id_fkey FOREIGN KEY (proposer_user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: trades trades_receiver_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trades
    ADD CONSTRAINT trades_receiver_user_id_fkey FOREIGN KEY (receiver_user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: user_passwords user_passwords_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_passwords
    ADD CONSTRAINT user_passwords_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_sessions user_sessions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_sessions
    ADD CONSTRAINT user_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_stations user_stations_station_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_stations
    ADD CONSTRAINT user_stations_station_id_fkey FOREIGN KEY (station_id) REFERENCES public.stations(id) ON DELETE RESTRICT;


--
-- Name: user_stations user_stations_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_stations
    ADD CONSTRAINT user_stations_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_two_factor_auths user_two_factor_auths_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_two_factor_auths
    ADD CONSTRAINT user_two_factor_auths_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_two_factor_recovery_codes user_two_factor_recovery_codes_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_two_factor_recovery_codes
    ADD CONSTRAINT user_two_factor_recovery_codes_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--



--
-- Dbmate schema migrations
--

INSERT INTO public.schema_migrations (version) VALUES
    ('20260917200138'),
    ('20260917200139'),
    ('20260917200140'),
    ('20260917204210'),
    ('20260918034816'),
    ('20260924033315'),
    ('20260924033316'),
    ('20260924060023'),
    ('20260924074856'),
    ('20260924101903'),
    ('20260924103149'),
    ('20260925035127'),
    ('20260925120000'),
    ('20260925120001'),
    ('20260925183659'),
    ('20260928054601'),
    ('20260928073147'),
    ('20260928073148'),
    ('20260928092841'),
    ('20260928092843'),
    ('20260928154225'),
    ('20260928161935'),
    ('20260929040100'),
    ('20260929045636'),
    ('20260929045637'),
    ('20260929045638'),
    ('20260929071059'),
    ('20260929074259'),
    ('20260929074300'),
    ('20260929074301'),
    ('20260929074302'),
    ('20260929162548');
