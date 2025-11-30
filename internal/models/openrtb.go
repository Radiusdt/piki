package models

import (
    "errors"
)

// Minimal but compatible subset of OpenRTB 2.5/2.6

type Device struct {
    Ua      string                 `json:"ua,omitempty"`
    IP      string                 `json:"ip,omitempty"`
    IPv6    string                 `json:"ipv6,omitempty"`
    OS      string                 `json:"os,omitempty"`
    OSV     string                 `json:"osv,omitempty"`
    DeviceType int32               `json:"devicetype,omitempty"`
    Make    string                 `json:"make,omitempty"`
    Model   string                 `json:"model,omitempty"`
    Ifa     string                 `json:"ifa,omitempty"`
    Lmt     int32                  `json:"lmt,omitempty"`
    Geo     map[string]interface{} `json:"geo,omitempty"`
}

type User struct {
    ID        string                 `json:"id,omitempty"`
    BuyerUID  string                 `json:"buyeruid,omitempty"`
    Yob       int32                  `json:"yob,omitempty"`
    Gender    string                 `json:"gender,omitempty"`
    Keywords  string                 `json:"keywords,omitempty"`
    CustomData string                `json:"customdata,omitempty"`
    Geo       map[string]interface{} `json:"geo,omitempty"`
}

type BannerFormat struct {
    W int32 `json:"w"`
    H int32 `json:"h"`
}

type Banner struct {
    W      int32          `json:"w,omitempty"`
    H      int32          `json:"h,omitempty"`
    Format []BannerFormat `json:"format,omitempty"`
    Mimes  []string       `json:"mimes,omitempty"`
    Pos    int32          `json:"pos,omitempty"`
}

// Video defines a video impression for OpenRTB.  It contains a subset
// of fields sufficient for this DSP to respond with video creatives.
// See OpenRTB spec 2.5 for full field definitions.
type Video struct {
    Mimes      []string `json:"mimes,omitempty"`
    W          int32    `json:"w,omitempty"`
    H          int32    `json:"h,omitempty"`
    MinDuration int32   `json:"minduration,omitempty"`
    MaxDuration int32   `json:"maxduration,omitempty"`
    Protocols  []int32  `json:"protocols,omitempty"`
}

type App struct {
    ID     string   `json:"id,omitempty"`
    Name   string   `json:"name,omitempty"`
    Bundle string   `json:"bundle,omitempty"`
    StoreURL string `json:"storeurl,omitempty"`
    Cat    []string `json:"cat,omitempty"`
}

type Site struct {
    ID     string   `json:"id,omitempty"`
    Name   string   `json:"name,omitempty"`
    Domain string   `json:"domain,omitempty"`
    Page   string   `json:"page,omitempty"`
    Cat    []string `json:"cat,omitempty"`
}

type Imp struct {
    ID         string                 `json:"id"`
    Banner     *Banner                `json:"banner,omitempty"`
    Video      *Video                 `json:"video,omitempty"`
    TagID      string                 `json:"tagid,omitempty"`
    BidFloor   float64                `json:"bidfloor,omitempty"`
    BidFloorCur string                `json:"bidfloorcur,omitempty"`
    Secure     int32                  `json:"secure,omitempty"`
    Instl      int32                  `json:"instl,omitempty"`
    Pmp        map[string]interface{} `json:"pmp,omitempty"`
    Ext        map[string]interface{} `json:"ext,omitempty"`
}

type BidRequest struct {
    ID   string                 `json:"id"`
    Imp  []Imp                  `json:"imp"`
    Site *Site                  `json:"site,omitempty"`
    App  *App                   `json:"app,omitempty"`
    Device *Device              `json:"device,omitempty"`
    User *User                  `json:"user,omitempty"`
    TMax int32                  `json:"tmax,omitempty"`
    At   int32                  `json:"at,omitempty"`
    Cur  []string               `json:"cur,omitempty"`
    BCat []string               `json:"bcat,omitempty"`
    BAdv []string               `json:"badv,omitempty"`
    Ext  map[string]interface{} `json:"ext,omitempty"`
}

func (br *BidRequest) Validate() error {
    if br.ID == "" {
        return errors.New("missing id")
    }
    if len(br.Imp) == 0 {
        return errors.New("missing imp")
    }
    return nil
}

// ---- RESPONSE ----

type Bid struct {
    ID      string                 `json:"id"`
    ImpID   string                 `json:"impid"`
    Price   float64                `json:"price"`
    CrID    string                 `json:"crid"`
    AdM     string                 `json:"adm,omitempty"`
    AdID    string                 `json:"adid,omitempty"`
    NURL    string                 `json:"nurl,omitempty"`
    LURL    string                 `json:"lurl,omitempty"`
    ADomain []string               `json:"adomain,omitempty"`
    CID     string                 `json:"cid,omitempty"`
    CrtrID  string                 `json:"crtrid,omitempty"`
    Ext     map[string]interface{} `json:"ext,omitempty"`
}

type SeatBid struct {
    Seat  string                 `json:"seat,omitempty"`
    Bid   []Bid                  `json:"bid"`
    Group int32                  `json:"group,omitempty"`
    Ext   map[string]interface{} `json:"ext,omitempty"`
}

type BidResponse struct {
    ID         string                 `json:"id"`
    SeatBid    []SeatBid              `json:"seatbid"`
    Cur        string                 `json:"cur,omitempty"`
    BidID      string                 `json:"bidid,omitempty"`
    CustomData string                 `json:"customdata,omitempty"`
    NBR        int32                  `json:"nbr,omitempty"`
    Ext        map[string]interface{} `json:"ext,omitempty"`
}