package youtube

import (
	"context"
	"net/url"
	"strings"

	"ytmusic/internal/config"
)

// AccountProfile contains the active YouTube channel identity and all linked Brand Accounts.
type AccountProfile struct {
	AccountName    string
	ChannelHandle  string
	Email          string
	AvatarURL      string
	ActivePageID   string
	AuthUser       string
	SessionExpired bool
	Accounts       []config.BrandAccount
}

// FetchAccountProfile queries InnerTube account/account_menu and account/accounts_list
// to retrieve the signed-in Google account identity and all available YouTube Brand Accounts.
func (c *Client) FetchAccountProfile(ctx context.Context) (AccountProfile, error) {
	c.mu.Lock()
	auth := c.auth
	c.mu.Unlock()

	profile := AccountProfile{
		ActivePageID: auth.ActivePageID,
		AuthUser:     auth.AuthUser,
	}
	if profile.AuthUser == "" {
		profile.AuthUser = "0"
	}

	var menuErr error
	menuResp, err := c.postInnerTube(ctx, "/youtubei/v1/account/account_menu?prettyPrint=false", map[string]any{
		"context":            c.buildContext(),
		"deviceTheme":        "DEVICE_THEME_SELECTED",
		"userInterfaceTheme": "USER_INTERFACE_THEME_DARK",
	})
	if err == nil {
		if isLoggedOutResponse(menuResp) {
			profile.SessionExpired = true
		} else {
			parseActiveAccountHeader(menuResp, &profile)
		}
	} else {
		menuErr = err
	}

	listResp, listErr := c.postInnerTube(ctx, "/youtubei/v1/account/accounts_list?prettyPrint=false", map[string]any{
		"context": c.buildContext(),
		"accountReadMask": map[string]any{
			"returnOwner": true,
		},
	})
	if listErr == nil {
		accounts := parseAccountsList(listResp, profile.Email)
		if len(accounts) > 0 {
			markAndMergeAccounts(accounts, &profile, auth.LoggedIn)
		} else if profile.AccountName == "" && isLoggedOutResponse(listResp) {
			profile.SessionExpired = true
		}
	}

	if profile.AccountName == "" && !profile.SessionExpired && menuErr != nil && listErr != nil {
		return profile, listErr
	}
	return profile, nil
}

func isLoggedOutResponse(root map[string]any) bool {
	if len(root) == 0 {
		return false
	}
	if rc, ok := root["responseContext"].(map[string]any); ok {
		if mainCtx, ok := rc["mainAppWebResponseContext"].(map[string]any); ok {
			if loggedOut, ok := mainCtx["loggedOut"].(bool); ok && loggedOut {
				return true
			}
		}
	}
	return false
}

func parseActiveAccountHeader(root any, out *AccountProfile) {
	var walk func(node any)
	walk = func(node any) {
		switch v := node.(type) {
		case map[string]any:
			if hdr, ok := v["activeAccountHeaderRenderer"].(map[string]any); ok {
				if name := extractText(hdr["accountName"]); name != "" {
					out.AccountName = name
				}
				if handle := extractText(hdr["channelHandle"]); handle != "" {
					out.ChannelHandle = handle
				}
				if email := extractText(hdr["email"]); email != "" {
					out.Email = email
				}
				if avatar := extractBestThumbnailURL(hdr["accountPhoto"]); avatar != "" {
					out.AvatarURL = avatar
				}
				return
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, item := range v {
				walk(item)
			}
		}
	}
	walk(root)
}

func parseAccountsList(root any, fallbackEmail string) []config.BrandAccount {
	var accounts []config.BrandAccount
	seenKey := make(map[string]struct{})

	var walk func(node any)
	walk = func(node any) {
		switch v := node.(type) {
		case map[string]any:
			if air, ok := v["accountItemRenderer"].(map[string]any); ok {
				if acc, ok := parseAccountItemRenderer(air, fallbackEmail); ok {
					key := acc.AuthUser + "|" + acc.PageID + "|" + strings.ToLower(acc.Name)
					if _, exists := seenKey[key]; !exists {
						seenKey[key] = struct{}{}
						accounts = append(accounts, acc)
					}
				}
				return
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, item := range v {
				walk(item)
			}
		}
	}
	walk(root)
	return accounts
}

func parseAccountItemRenderer(m map[string]any, fallbackEmail string) (config.BrandAccount, bool) {
	name := extractText(m["accountName"])
	if name == "" {
		return config.BrandAccount{}, false
	}

	selected, _ := m["isSelected"].(bool)
	byline := extractText(m["accountByline"])
	handle := extractText(m["channelHandle"])
	avatar := extractBestThumbnailURL(m["accountPhoto"])

	var email, subscriberTxt string
	if byline != "" {
		switch {
		case strings.HasPrefix(byline, "@"):
			if handle == "" {
				handle = byline
			}
		case strings.Contains(byline, "@") && strings.Contains(byline, "."):
			email = byline
		default:
			subscriberTxt = byline
		}
	}
	if email == "" && fallbackEmail != "" {
		email = fallbackEmail
	}

	pageID, authUser := extractIdentityTokens(m)
	return config.BrandAccount{
		Name:          name,
		Handle:        handle,
		Email:         email,
		AvatarURL:     avatar,
		PageID:        pageID,
		AuthUser:      authUser,
		SubscriberTxt: subscriberTxt,
		Selected:      selected,
	}, true
}

func extractIdentityTokens(m map[string]any) (pageID, authUser string) {
	var walk func(node any)
	walk = func(node any) {
		switch v := node.(type) {
		case map[string]any:
			if tok, ok := v["pageIdToken"].(map[string]any); ok && pageID == "" {
				if pid, _ := tok["pageId"].(string); pid != "" {
					pageID = pid
				}
			}
			if tok, ok := v["datasyncIdToken"].(map[string]any); ok && pageID == "" {
				if raw, _ := tok["datasyncIdToken"].(string); strings.Contains(raw, "||") {
					parts := strings.SplitN(raw, "||", 2)
					if parts[0] != "" && parts[0] != parts[1] {
						pageID = parts[0]
					}
				}
			}
			if tok, ok := v["accountSigninToken"].(map[string]any); ok && authUser == "" {
				if rawURL, _ := tok["signinUrl"].(string); rawURL != "" {
					if u, err := url.Parse(rawURL); err == nil {
						if au := u.Query().Get("authuser"); au != "" {
							authUser = au
						}
					}
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, item := range v {
				walk(item)
			}
		}
	}
	walk(m["serviceEndpoint"])
	if pageID == "" || authUser == "" {
		walk(m["navigationEndpoint"])
	}
	return pageID, authUser
}

func markAndMergeAccounts(accounts []config.BrandAccount, profile *AccountProfile, forcePageIDMatch bool) {
	selectedIdx := -1
	ownerEmail := profile.Email
	fellBackToPrimary := false
	for i := range accounts {
		if ownerEmail == "" && accounts[i].Email != "" {
			ownerEmail = accounts[i].Email
		}
		if profile.ActivePageID != "" || forcePageIDMatch {
			matched := accounts[i].PageID == profile.ActivePageID
			if matched && profile.ActivePageID == "" && profile.AuthUser != "" && accounts[i].AuthUser != "" {
				matched = accounts[i].AuthUser == profile.AuthUser
			}
			accounts[i].Selected = matched
			if accounts[i].Selected && selectedIdx < 0 {
				selectedIdx = i
			} else {
				accounts[i].Selected = false
			}
		} else {
			if accounts[i].Selected && selectedIdx < 0 {
				selectedIdx = i
			} else {
				accounts[i].Selected = false
			}
		}
	}

	if selectedIdx < 0 {
		// Primary account corresponds to empty PageID when no delegated session is set.
		for i := range accounts {
			if accounts[i].PageID == "" && (profile.AuthUser == "" || accounts[i].AuthUser == "" || accounts[i].AuthUser == profile.AuthUser) {
				accounts[i].Selected = true
				selectedIdx = i
				fellBackToPrimary = true
				break
			}
		}
		if selectedIdx < 0 {
			for i := range accounts {
				if accounts[i].PageID == "" {
					accounts[i].Selected = true
					selectedIdx = i
					fellBackToPrimary = true
					break
				}
			}
		}
	}

	if ownerEmail != "" {
		if profile.Email == "" {
			profile.Email = ownerEmail
		}
		for i := range accounts {
			if accounts[i].Email == "" {
				accounts[i].Email = ownerEmail
			}
		}
	}

	if selectedIdx >= 0 {
		sel := accounts[selectedIdx]
		if (profile.AccountName == "" || forcePageIDMatch) && sel.Name != "" {
			profile.AccountName = sel.Name
		}
		if (profile.ChannelHandle == "" || forcePageIDMatch) && sel.Handle != "" {
			profile.ChannelHandle = sel.Handle
		}
		if (profile.AvatarURL == "" || forcePageIDMatch) && sel.AvatarURL != "" {
			profile.AvatarURL = sel.AvatarURL
		}
		if (profile.ActivePageID == "" && !forcePageIDMatch) || fellBackToPrimary {
			profile.ActivePageID = sel.PageID
		}
		if sel.AuthUser != "" {
			profile.AuthUser = sel.AuthUser
		}
	}
	profile.Accounts = accounts
}
