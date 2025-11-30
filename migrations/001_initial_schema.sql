-- Vector-DSP Database Schema
-- PostgreSQL Migration v001

-- =============================================
-- ADVERTISERS
-- =============================================

CREATE TABLE IF NOT EXISTS advertisers (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    contact_email VARCHAR(255),
    contact_name VARCHAR(255),
    status VARCHAR(20) DEFAULT 'active',
    balance DECIMAL(15,4) DEFAULT 0,
    currency VARCHAR(3) DEFAULT 'USD',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_advertisers_status ON advertisers(status);

-- =============================================
-- CAMPAIGNS
-- =============================================

CREATE TABLE IF NOT EXISTS campaigns (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    advertiser_id VARCHAR(64) REFERENCES advertisers(id),
    status VARCHAR(20) DEFAULT 'draft',
    
    -- App info
    app_bundle VARCHAR(255),
    app_name VARCHAR(255),
    app_store_url TEXT,
    
    -- MMP Configuration
    mmp_type VARCHAR(20), -- appsflyer, adjust, singular, branch, kochava
    mmp_click_url TEXT,
    mmp_view_url TEXT,
    mmp_macros_mapping JSONB DEFAULT '{}',
    mmp_postback_events TEXT[], -- array of events: install, registration, purchase
    
    -- Payout
    payout_type VARCHAR(20) DEFAULT 'fixed', -- fixed, percent, dynamic
    payout_amount DECIMAL(10,4) DEFAULT 0,
    payout_event VARCHAR(50) DEFAULT 'install',
    
    -- Budget
    daily_budget DECIMAL(15,4),
    total_budget DECIMAL(15,4),
    daily_cap INTEGER,
    total_cap INTEGER,
    
    -- Schedule
    start_date TIMESTAMP,
    end_date TIMESTAMP,
    timezone VARCHAR(50) DEFAULT 'UTC',
    
    -- Targeting (stored as JSONB for flexibility)
    targeting JSONB DEFAULT '{}',
    
    -- Metadata
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_campaigns_advertiser ON campaigns(advertiser_id);
CREATE INDEX idx_campaigns_status ON campaigns(status);
CREATE INDEX idx_campaigns_mmp_type ON campaigns(mmp_type);

-- =============================================
-- LINE ITEMS
-- =============================================

CREATE TABLE IF NOT EXISTS line_items (
    id VARCHAR(64) PRIMARY KEY,
    campaign_id VARCHAR(64) REFERENCES campaigns(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(20) DEFAULT 'active',
    
    -- Bidding
    bid_type VARCHAR(20) DEFAULT 'cpm', -- cpm, cpc, cpi
    bid_amount DECIMAL(10,4),
    bid_strategy VARCHAR(20) DEFAULT 'fixed', -- fixed, optimize
    max_bid DECIMAL(10,4),
    min_bid DECIMAL(10,4),
    
    -- Budget
    daily_budget DECIMAL(15,4),
    total_budget DECIMAL(15,4),
    daily_cap INTEGER,
    total_cap INTEGER,
    
    -- Pacing
    pacing_type VARCHAR(20) DEFAULT 'even', -- even, asap
    pacing_distribution JSONB, -- hourly distribution
    
    -- Targeting override
    targeting JSONB DEFAULT '{}',
    
    -- Frequency cap
    freq_cap_impressions INTEGER,
    freq_cap_period_hours INTEGER DEFAULT 24,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_line_items_campaign ON line_items(campaign_id);
CREATE INDEX idx_line_items_status ON line_items(status);

-- =============================================
-- CREATIVES
-- =============================================

CREATE TABLE IF NOT EXISTS creatives (
    id VARCHAR(64) PRIMARY KEY,
    advertiser_id VARCHAR(64) REFERENCES advertisers(id),
    name VARCHAR(255),
    format VARCHAR(20) NOT NULL, -- banner, video, native, audio
    status VARCHAR(20) DEFAULT 'pending', -- pending, approved, rejected
    
    -- Dimensions
    width INTEGER,
    height INTEGER,
    
    -- Content
    adm_template TEXT, -- ad markup template
    banner_url TEXT,
    video_url TEXT,
    video_duration INTEGER,
    
    -- Native specific
    title VARCHAR(255),
    description TEXT,
    icon_url TEXT,
    image_url TEXT,
    cta_text VARCHAR(50),
    sponsored_by VARCHAR(100),
    rating DECIMAL(2,1),
    
    -- Metadata
    mime_types TEXT[],
    iab_categories TEXT[],
    attr INTEGER[],
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_creatives_advertiser ON creatives(advertiser_id);
CREATE INDEX idx_creatives_format ON creatives(format);
CREATE INDEX idx_creatives_status ON creatives(status);

-- =============================================
-- LINE ITEM CREATIVES (many-to-many)
-- =============================================

CREATE TABLE IF NOT EXISTS line_item_creatives (
    line_item_id VARCHAR(64) REFERENCES line_items(id) ON DELETE CASCADE,
    creative_id VARCHAR(64) REFERENCES creatives(id) ON DELETE CASCADE,
    weight INTEGER DEFAULT 100,
    status VARCHAR(20) DEFAULT 'active',
    PRIMARY KEY (line_item_id, creative_id)
);

-- =============================================
-- S2S SOURCES (Direct Traffic Partners)
-- =============================================

CREATE TABLE IF NOT EXISTS s2s_sources (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    internal_name VARCHAR(100) UNIQUE NOT NULL, -- for URL: /s2s/{internal_name}/ad
    status VARCHAR(20) DEFAULT 'active',
    
    -- Authentication
    api_token VARCHAR(255),
    allowed_ips TEXT[], -- IP whitelist
    
    -- Postback configuration
    postback_url TEXT,
    postback_method VARCHAR(10) DEFAULT 'GET',
    postback_events TEXT[], -- which events to send: install, registration, purchase
    macros_mapping JSONB DEFAULT '{}', -- custom macros: {"click_id": "clickid"}
    
    -- Default payout (can be overridden per campaign)
    default_payout DECIMAL(10,4),
    default_payout_type VARCHAR(20) DEFAULT 'fixed',
    
    -- Caps
    daily_cap INTEGER,
    monthly_cap INTEGER,
    
    -- Stats
    total_clicks BIGINT DEFAULT 0,
    total_conversions BIGINT DEFAULT 0,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_s2s_sources_internal_name ON s2s_sources(internal_name);
CREATE INDEX idx_s2s_sources_status ON s2s_sources(status);

-- =============================================
-- RTB SOURCES (SSPs / Exchanges)
-- =============================================

CREATE TABLE IF NOT EXISTS rtb_sources (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(20) DEFAULT 'active',
    
    -- Endpoint (where we send bid requests from, or receive from)
    endpoint_url TEXT,
    protocol_version VARCHAR(10) DEFAULT '2.5', -- OpenRTB version
    
    -- Supported formats
    supported_formats TEXT[], -- banner, video, native, audio
    supported_sizes JSONB, -- [{"w": 320, "h": 50}, ...]
    
    -- Bidding adjustments
    bid_adjustment DECIMAL(5,2) DEFAULT 1.0, -- multiplier
    bid_floor_adjustment DECIMAL(5,2) DEFAULT 0,
    
    -- QPS limits
    max_qps INTEGER DEFAULT 1000,
    timeout_ms INTEGER DEFAULT 100,
    
    -- Authentication
    auth_type VARCHAR(20), -- none, token, basic
    auth_token VARCHAR(255),
    
    -- Win notification
    nurl_required BOOLEAN DEFAULT true,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_rtb_sources_status ON rtb_sources(status);

-- =============================================
-- CAMPAIGN SOURCES (linking campaigns to sources)
-- =============================================

CREATE TABLE IF NOT EXISTS campaign_sources (
    id VARCHAR(64) PRIMARY KEY,
    campaign_id VARCHAR(64) REFERENCES campaigns(id) ON DELETE CASCADE,
    source_type VARCHAR(10) NOT NULL, -- s2s, rtb
    source_id VARCHAR(64) NOT NULL,
    status VARCHAR(20) DEFAULT 'active',
    
    -- Custom payout for this campaign-source pair
    custom_payout DECIMAL(10,4),
    custom_payout_type VARCHAR(20),
    
    -- Custom bid
    custom_bid DECIMAL(10,4),
    
    -- Caps
    daily_cap INTEGER,
    total_cap INTEGER,
    
    -- Stats
    clicks BIGINT DEFAULT 0,
    conversions BIGINT DEFAULT 0,
    spend DECIMAL(15,4) DEFAULT 0,
    revenue DECIMAL(15,4) DEFAULT 0,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(campaign_id, source_type, source_id)
);

CREATE INDEX idx_campaign_sources_campaign ON campaign_sources(campaign_id);
CREATE INDEX idx_campaign_sources_source ON campaign_sources(source_type, source_id);

-- =============================================
-- CLICKS
-- =============================================

CREATE TABLE IF NOT EXISTS clicks (
    id VARCHAR(64) PRIMARY KEY,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Campaign info
    campaign_id VARCHAR(64),
    line_item_id VARCHAR(64),
    creative_id VARCHAR(64),
    
    -- Source info
    source_type VARCHAR(10), -- s2s, rtb
    source_id VARCHAR(64),
    
    -- Device info
    device_ifa VARCHAR(64), -- GAID or IDFA
    ip VARCHAR(45),
    user_agent TEXT,
    
    -- Geo
    geo_country VARCHAR(2),
    geo_region VARCHAR(10),
    geo_city VARCHAR(100),
    
    -- Device
    device_type VARCHAR(20),
    device_os VARCHAR(20),
    device_osv VARCHAR(20),
    device_make VARCHAR(50),
    device_model VARCHAR(50),
    
    -- RTB specific
    bid_request_id VARCHAR(64),
    bid_price DECIMAL(10,6),
    win_price DECIMAL(10,6),
    
    -- Sub IDs (from source)
    sub1 VARCHAR(255),
    sub2 VARCHAR(255),
    sub3 VARCHAR(255),
    sub4 VARCHAR(255),
    sub5 VARCHAR(255),
    
    -- Target URL (MMP URL after macro replacement)
    target_url TEXT,
    
    -- Additional params as JSON
    params JSONB DEFAULT '{}'
);

CREATE INDEX idx_clicks_timestamp ON clicks(timestamp);
CREATE INDEX idx_clicks_campaign ON clicks(campaign_id);
CREATE INDEX idx_clicks_source ON clicks(source_type, source_id);
CREATE INDEX idx_clicks_device_ifa ON clicks(device_ifa);
CREATE INDEX idx_clicks_geo ON clicks(geo_country);

-- Partition by month for better performance (optional)
-- CREATE TABLE clicks_2025_01 PARTITION OF clicks FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');

-- =============================================
-- IMPRESSIONS
-- =============================================

CREATE TABLE IF NOT EXISTS impressions (
    id VARCHAR(64) PRIMARY KEY,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Campaign info
    campaign_id VARCHAR(64),
    line_item_id VARCHAR(64),
    creative_id VARCHAR(64),
    
    -- Source info
    source_type VARCHAR(10),
    source_id VARCHAR(64),
    
    -- RTB specific
    bid_request_id VARCHAR(64),
    bid_price DECIMAL(10,6),
    win_price DECIMAL(10,6),
    
    -- Device
    device_ifa VARCHAR(64),
    ip VARCHAR(45),
    geo_country VARCHAR(2),
    device_os VARCHAR(20),
    
    -- Publisher
    app_bundle VARCHAR(255),
    publisher_id VARCHAR(64)
);

CREATE INDEX idx_impressions_timestamp ON impressions(timestamp);
CREATE INDEX idx_impressions_campaign ON impressions(campaign_id);

-- =============================================
-- CONVERSIONS (Postbacks from MMP)
-- =============================================

CREATE TABLE IF NOT EXISTS conversions (
    id VARCHAR(64) PRIMARY KEY,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Click reference
    click_id VARCHAR(64),
    click_timestamp TIMESTAMP,
    
    -- Source info (denormalized for reporting)
    source_type VARCHAR(10),
    source_id VARCHAR(64),
    
    -- Campaign info (denormalized)
    campaign_id VARCHAR(64),
    line_item_id VARCHAR(64),
    creative_id VARCHAR(64),
    
    -- Event info
    event VARCHAR(50) NOT NULL, -- internal event name: install, registration, purchase
    event_original VARCHAR(100), -- original event from MMP: af_purchase
    
    -- Revenue
    revenue DECIMAL(15,4),
    revenue_currency VARCHAR(3),
    revenue_usd DECIMAL(15,4),
    
    -- Payout
    payout DECIMAL(15,4),
    payout_currency VARCHAR(3) DEFAULT 'USD',
    payout_usd DECIMAL(15,4),
    
    -- Device (from click)
    device_ifa VARCHAR(64),
    geo_country VARCHAR(2),
    
    -- Attribution
    time_to_install INTEGER, -- seconds from click to install
    
    -- External reference
    external_id VARCHAR(255), -- MMP's conversion ID
    
    -- Additional params
    params JSONB DEFAULT '{}'
);

CREATE INDEX idx_conversions_timestamp ON conversions(timestamp);
CREATE INDEX idx_conversions_click ON conversions(click_id);
CREATE INDEX idx_conversions_campaign ON conversions(campaign_id);
CREATE INDEX idx_conversions_source ON conversions(source_type, source_id);
CREATE INDEX idx_conversions_event ON conversions(event);

-- =============================================
-- DAILY STATS (Aggregated for reporting)
-- =============================================

CREATE TABLE IF NOT EXISTS daily_stats (
    date DATE NOT NULL,
    campaign_id VARCHAR(64),
    line_item_id VARCHAR(64),
    creative_id VARCHAR(64),
    source_type VARCHAR(10),
    source_id VARCHAR(64),
    geo_country VARCHAR(2),
    device_os VARCHAR(20),
    
    -- Metrics
    impressions BIGINT DEFAULT 0,
    clicks BIGINT DEFAULT 0,
    installs BIGINT DEFAULT 0,
    events BIGINT DEFAULT 0,
    
    spend DECIMAL(15,4) DEFAULT 0,
    revenue DECIMAL(15,4) DEFAULT 0,
    payout DECIMAL(15,4) DEFAULT 0,
    
    PRIMARY KEY (date, campaign_id, COALESCE(line_item_id, ''), COALESCE(source_type, ''), COALESCE(source_id, ''), COALESCE(geo_country, ''))
);

CREATE INDEX idx_daily_stats_date ON daily_stats(date);
CREATE INDEX idx_daily_stats_campaign ON daily_stats(campaign_id);

-- =============================================
-- API KEYS
-- =============================================

CREATE TABLE IF NOT EXISTS api_keys (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) NOT NULL UNIQUE,
    advertiser_id VARCHAR(64) REFERENCES advertisers(id),
    permissions TEXT[], -- read, write, admin
    rate_limit INTEGER DEFAULT 100,
    status VARCHAR(20) DEFAULT 'active',
    last_used_at TIMESTAMP,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);

-- =============================================
-- FUNCTIONS
-- =============================================

-- Update timestamp function
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply to tables
CREATE TRIGGER update_advertisers_timestamp BEFORE UPDATE ON advertisers FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER update_campaigns_timestamp BEFORE UPDATE ON campaigns FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER update_line_items_timestamp BEFORE UPDATE ON line_items FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER update_creatives_timestamp BEFORE UPDATE ON creatives FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER update_s2s_sources_timestamp BEFORE UPDATE ON s2s_sources FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER update_rtb_sources_timestamp BEFORE UPDATE ON rtb_sources FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER update_campaign_sources_timestamp BEFORE UPDATE ON campaign_sources FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- =============================================
-- SEED DATA (Optional)
-- =============================================

-- Example advertiser
INSERT INTO advertisers (id, name, contact_email, status, balance) 
VALUES ('adv-001', 'Demo Advertiser', 'demo@example.com', 'active', 10000.00)
ON CONFLICT (id) DO NOTHING;

-- Example S2S source
INSERT INTO s2s_sources (id, name, internal_name, status, default_payout, postback_url, postback_events)
VALUES ('src-s2s-001', 'Demo S2S Partner', 'demo_partner', 'active', 0.50, 
        'https://partner.example.com/postback?click_id={click_id}&event={event}&payout={payout}',
        ARRAY['install', 'registration'])
ON CONFLICT (id) DO NOTHING;
