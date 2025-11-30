-- Vector-DSP Initial Schema
-- PostgreSQL 14+

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================
-- ADVERTISERS
-- ============================================
CREATE TABLE IF NOT EXISTS advertisers (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    legal_name VARCHAR(255),
    tax_id VARCHAR(64),
    address TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_advertisers_name ON advertisers(name);

-- ============================================
-- CAMPAIGNS
-- ============================================
CREATE TABLE IF NOT EXISTS campaigns (
    id VARCHAR(64) PRIMARY KEY,
    advertiser_id VARCHAR(64) NOT NULL REFERENCES advertisers(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'paused',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_campaign_status CHECK (status IN ('active', 'paused', 'ended'))
);

CREATE INDEX idx_campaigns_advertiser ON campaigns(advertiser_id);
CREATE INDEX idx_campaigns_status ON campaigns(status);

-- ============================================
-- LINE ITEMS
-- ============================================
CREATE TABLE IF NOT EXISTS line_items (
    id VARCHAR(64) PRIMARY KEY,
    campaign_id VARCHAR(64) NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT false,
    priority INT NOT NULL DEFAULT 0,
    
    -- Bid Strategy
    bid_strategy_type VARCHAR(32) NOT NULL DEFAULT 'fixed_cpm',
    fixed_cpm DECIMAL(10, 4),
    
    -- Targeting (stored as JSONB for flexibility)
    targeting JSONB NOT NULL DEFAULT '{}',
    
    -- Pacing
    daily_budget DECIMAL(12, 2) NOT NULL,
    total_budget DECIMAL(12, 2),
    start_at TIMESTAMPTZ,
    end_at TIMESTAMPTZ,
    freq_cap_per_user_per_day INT DEFAULT 0,
    qps_limit_per_source INT DEFAULT 0,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_line_items_campaign ON line_items(campaign_id);
CREATE INDEX idx_line_items_active ON line_items(is_active) WHERE is_active = true;

-- ============================================
-- CREATIVES
-- ============================================
CREATE TABLE IF NOT EXISTS creatives (
    id VARCHAR(64) PRIMARY KEY,
    advertiser_id VARCHAR(64) REFERENCES advertisers(id) ON DELETE SET NULL,
    line_item_id VARCHAR(64) REFERENCES line_items(id) ON DELETE CASCADE,
    
    format VARCHAR(32) NOT NULL DEFAULT 'banner',
    adm_template TEXT,
    width INT NOT NULL DEFAULT 0,
    height INT NOT NULL DEFAULT 0,
    adomain TEXT[], -- array of advertiser domains
    click_url TEXT,
    
    -- Video specific
    video_url TEXT,
    vast_tag TEXT,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_creative_format CHECK (format IN ('banner', 'video', 'native'))
);

CREATE INDEX idx_creatives_advertiser ON creatives(advertiser_id);
CREATE INDEX idx_creatives_line_item ON creatives(line_item_id);
CREATE INDEX idx_creatives_format ON creatives(format);

-- ============================================
-- AD GROUPS
-- ============================================
CREATE TABLE IF NOT EXISTS ad_groups (
    id VARCHAR(64) PRIMARY KEY,
    campaign_id VARCHAR(64) NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    budget DECIMAL(12, 2) NOT NULL,
    start_at TIMESTAMPTZ,
    end_at TIMESTAMPTZ,
    targeting JSONB NOT NULL DEFAULT '{}',
    creative_ids TEXT[], -- array of creative IDs
    is_active BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ad_groups_campaign ON ad_groups(campaign_id);

-- ============================================
-- CLICKS
-- ============================================
CREATE TABLE IF NOT EXISTS clicks (
    id VARCHAR(64) PRIMARY KEY,
    campaign_id VARCHAR(64) NOT NULL,
    line_item_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(255),
    target_url TEXT NOT NULL,
    ip_address INET,
    user_agent TEXT,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_clicks_campaign ON clicks(campaign_id);
CREATE INDEX idx_clicks_line_item ON clicks(line_item_id);
CREATE INDEX idx_clicks_timestamp ON clicks(timestamp);

-- Partition clicks by month for performance (optional, requires PG 11+)
-- CREATE TABLE clicks_partitioned (...) PARTITION BY RANGE (timestamp);

-- ============================================
-- CONVERSIONS
-- ============================================
CREATE TABLE IF NOT EXISTS conversions (
    id VARCHAR(64) PRIMARY KEY,
    click_id VARCHAR(64) REFERENCES clicks(id) ON DELETE SET NULL,
    external_id VARCHAR(255),
    event_name VARCHAR(64) NOT NULL DEFAULT 'install',
    revenue DECIMAL(12, 4) DEFAULT 0,
    currency VARCHAR(3) DEFAULT 'USD',
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_conversions_click ON conversions(click_id);
CREATE INDEX idx_conversions_external ON conversions(external_id);
CREATE INDEX idx_conversions_timestamp ON conversions(timestamp);

-- ============================================
-- IMPRESSIONS (for tracking wins)
-- ============================================
CREATE TABLE IF NOT EXISTS impressions (
    id VARCHAR(64) PRIMARY KEY,
    campaign_id VARCHAR(64) NOT NULL,
    line_item_id VARCHAR(64) NOT NULL,
    creative_id VARCHAR(64),
    bid_id VARCHAR(64),
    imp_id VARCHAR(64),
    price DECIMAL(10, 6) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    user_id VARCHAR(255),
    ip_address INET,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_impressions_campaign ON impressions(campaign_id);
CREATE INDEX idx_impressions_line_item ON impressions(line_item_id);
CREATE INDEX idx_impressions_timestamp ON impressions(timestamp);

-- ============================================
-- API KEYS (for per-advertiser auth)
-- ============================================
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key_hash VARCHAR(64) NOT NULL UNIQUE, -- SHA256 hash of the key
    advertiser_id VARCHAR(64) REFERENCES advertisers(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    permissions TEXT[] DEFAULT ARRAY['read'], -- read, write, admin
    is_active BOOLEAN NOT NULL DEFAULT true,
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_advertiser ON api_keys(advertiser_id);

-- ============================================
-- FUNCTIONS
-- ============================================

-- Auto-update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply trigger to all tables with updated_at
CREATE TRIGGER update_advertisers_updated_at BEFORE UPDATE ON advertisers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_campaigns_updated_at BEFORE UPDATE ON campaigns
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_line_items_updated_at BEFORE UPDATE ON line_items
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_creatives_updated_at BEFORE UPDATE ON creatives
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_ad_groups_updated_at BEFORE UPDATE ON ad_groups
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================
-- VIEWS (for reporting)
-- ============================================

-- Campaign stats view
CREATE OR REPLACE VIEW campaign_stats AS
SELECT 
    c.id AS campaign_id,
    c.name AS campaign_name,
    c.status,
    COUNT(DISTINCT i.id) AS impressions,
    COUNT(DISTINCT cl.id) AS clicks,
    COUNT(DISTINCT cv.id) AS conversions,
    COALESCE(SUM(i.price), 0) AS spend,
    COALESCE(SUM(cv.revenue), 0) AS revenue,
    CASE WHEN COUNT(DISTINCT i.id) > 0 
        THEN ROUND(COUNT(DISTINCT cl.id)::DECIMAL / COUNT(DISTINCT i.id) * 100, 2) 
        ELSE 0 
    END AS ctr,
    CASE WHEN COUNT(DISTINCT cl.id) > 0 
        THEN ROUND(COUNT(DISTINCT cv.id)::DECIMAL / COUNT(DISTINCT cl.id) * 100, 2) 
        ELSE 0 
    END AS conversion_rate
FROM campaigns c
LEFT JOIN impressions i ON c.id = i.campaign_id
LEFT JOIN clicks cl ON c.id = cl.campaign_id
LEFT JOIN conversions cv ON cl.id = cv.click_id
GROUP BY c.id, c.name, c.status;
