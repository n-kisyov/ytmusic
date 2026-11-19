package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ytmusic/internal/config"
)

func TestParseDurationSeconds(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"3:45", 225},
		{"0:30", 30},
		{"1:02:03", 3723},
		{"LIVE", 0},
		{"", 0},
		{"10", 0},
	}
	for _, tc := range cases {
		if got := ParseDurationSeconds(tc.in); got != tc.want {
			t.Errorf("ParseDurationSeconds(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestComputeSAPISIDHash(t *testing.T) {
	got := ComputeSAPISIDHash("test_sapisid", "https://www.youtube.com", 1700000000)
	if !strings.HasPrefix(got, "SAPISIDHASH 1700000000_") || len(got) != len("SAPISIDHASH 1700000000_")+40 {
		t.Fatalf("unexpected SAPISIDHASH format: %q", got)
	}
}

func TestFetchAccountProfileWithBrandAccounts(t *testing.T) {
	var seenPageHeader string
	var seenOnBehalf string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPageHeader = r.Header.Get("X-Goog-PageId")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if ctxMap, ok := body["context"].(map[string]any); ok {
			if userMap, ok := ctxMap["user"].(map[string]any); ok {
				seenOnBehalf, _ = userMap["onBehalfOfUser"].(string)
			}
		}

		var resp map[string]any
		switch {
		case strings.Contains(r.URL.Path, "account_menu"):
			resp = map[string]any{
				"actions": []any{
					map[string]any{
						"openPopupAction": map[string]any{
							"popup": map[string]any{
								"multiPageMenuRenderer": map[string]any{
									"header": map[string]any{
										"activeAccountHeaderRenderer": map[string]any{
											"accountName":   map[string]any{"simpleText": "Studio Brand"},
											"channelHandle": map[string]any{"simpleText": "@studiobrand"},
											"email":         map[string]any{"simpleText": "owner@example.com"},
											"accountPhoto": map[string]any{
												"thumbnails": []any{
													map[string]any{"url": "https://yt3.ggpht.com/brand.jpg"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}
		case strings.Contains(r.URL.Path, "accounts_list"):
			resp = map[string]any{
				"actions": []any{
					map[string]any{
						"getMultiPageMenuAction": map[string]any{
							"menu": map[string]any{
								"multiPageMenuRenderer": map[string]any{
									"sections": []any{
										map[string]any{
											"accountSectionListRenderer": map[string]any{
												"contents": []any{
													map[string]any{
														"accountItemSectionRenderer": map[string]any{
															"contents": []any{
																map[string]any{
																	"accountItemRenderer": map[string]any{
																		"accountName":   map[string]any{"simpleText": "Personal Owner"},
																		"accountByline": map[string]any{"simpleText": "owner@example.com"},
																		"isSelected":    false,
																		"accountPhoto": map[string]any{
																			"thumbnails": []any{map[string]any{"url": "https://yt3.ggpht.com/owner.jpg"}},
																		},
																		"serviceEndpoint": map[string]any{
																			"selectActiveIdentityEndpoint": map[string]any{
																				"supportedTokens": []any{
																					map[string]any{"datasyncIdToken": map[string]any{"datasyncIdToken": "||1122334455"}},
																				},
																			},
																		},
																	},
																},
																map[string]any{
																	"accountItemRenderer": map[string]any{
																		"accountName":   map[string]any{"simpleText": "Studio Brand"},
																		"accountByline": map[string]any{"simpleText": "1.2K subscribers"},
																		"channelHandle": map[string]any{"simpleText": "@studiobrand"},
																		"isSelected":    true,
																		"accountPhoto": map[string]any{
																			"thumbnails": []any{map[string]any{"url": "https://yt3.ggpht.com/brand.jpg"}},
																		},
																		"serviceEndpoint": map[string]any{
																			"selectActiveIdentityEndpoint": map[string]any{
																				"supportedTokens": []any{
																					map[string]any{"pageIdToken": map[string]any{"pageId": "109876543210"}},
																				},
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.Client())
	client.SetBaseURL(srv.URL)
	client.SetAuth(config.AuthState{
		LoggedIn:     true,
		ActivePageID: "109876543210",
		Cookies:      map[string]string{"SAPISID": "secret"},
	})

	prof, err := client.FetchAccountProfile(context.Background())
	if err != nil {
		t.Fatalf("FetchAccountProfile failed: %v", err)
	}
	if seenPageHeader != "109876543210" || seenOnBehalf != "109876543210" {
		t.Fatalf("expected delegation headers/context set to 109876543210, got header=%q ctx=%q", seenPageHeader, seenOnBehalf)
	}
	if prof.AccountName != "Studio Brand" || prof.ChannelHandle != "@studiobrand" || prof.Email != "owner@example.com" {
		t.Fatalf("unexpected profile identity: %+v", prof)
	}
	if len(prof.Accounts) != 2 || prof.Accounts[0].PageID != "" || prof.Accounts[1].PageID != "109876543210" || !prof.Accounts[1].Selected {
		t.Fatalf("unexpected parsed brand accounts: %+v", prof.Accounts)
	}
}

func TestSearchInitialPlaylistCollects30AndLoadMoreAppends(t *testing.T) {
	makeVideo := func(i int) map[string]any {
		return map[string]any{
			"videoRenderer": map[string]any{
				"videoId": fmt.Sprintf("vid_%02d", i),
				"title": map[string]any{
					"runs": []any{map[string]any{"text": fmt.Sprintf("Song %02d", i)}},
				},
				"ownerText": map[string]any{
					"runs": []any{map[string]any{"text": fmt.Sprintf("Artist %02d", i)}},
				},
				"lengthText": map[string]any{
					"simpleText": "3:15",
				},
			},
		}
	}

	page1Items := make([]any, 0, 19)
	for i := 1; i <= 18; i++ {
		page1Items = append(page1Items, makeVideo(i))
	}
	// Live item without lengthText should be filtered out.
	page1Items = append(page1Items, map[string]any{
		"videoRenderer": map[string]any{
			"videoId": "live_stream",
			"title":   map[string]any{"simpleText": "24/7 Live Radio"},
		},
	})
	page1Items = append(page1Items, map[string]any{
		"continuationItemRenderer": map[string]any{
			"continuationEndpoint": map[string]any{
				"continuationCommand": map[string]any{"token": "CONT_PAGE_2"},
			},
		},
	})

	page2Items := make([]any, 0, 18)
	for i := 19; i <= 35; i++ {
		page2Items = append(page2Items, makeVideo(i))
	}
	page2Items = append(page2Items, map[string]any{
		"continuationItemRenderer": map[string]any{
			"continuationEndpoint": map[string]any{
				"continuationCommand": map[string]any{"token": "CONT_PAGE_3"},
			},
		},
	})

	page3Items := make([]any, 0, 16)
	for i := 36; i <= 50; i++ {
		page3Items = append(page3Items, makeVideo(i))
	}
	page3Items = append(page3Items, map[string]any{
		"continuationItemRenderer": map[string]any{
			"continuationEndpoint": map[string]any{
				"continuationCommand": map[string]any{"token": "CONT_PAGE_4"},
			},
		},
	})

	page4Items := make([]any, 0, 10)
	for i := 51; i <= 60; i++ {
		page4Items = append(page4Items, makeVideo(i))
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		cont, _ := body["continuation"].(string)

		var resp map[string]any
		switch cont {
		case "":
			resp = map[string]any{"contents": page1Items}
		case "CONT_PAGE_2":
			resp = map[string]any{"onResponseReceivedCommands": page2Items}
		case "CONT_PAGE_3":
			resp = map[string]any{"onResponseReceivedCommands": page3Items}
		case "CONT_PAGE_4":
			resp = map[string]any{"onResponseReceivedCommands": page4Items}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.Client())
	client.SetBaseURL(srv.URL)
	client.SetAuth(config.AuthState{
		LoggedIn: true,
		Cookies:  map[string]string{"SAPISID": "secret"},
	})

	res, err := client.SearchInitialPlaylist(context.Background(), "synthwave", 30)
	if err != nil {
		t.Fatalf("SearchInitialPlaylist failed: %v", err)
	}
	if len(res.Tracks) != 30 {
		t.Fatalf("expected exactly 30 tracks, got %d", len(res.Tracks))
	}
	if res.Tracks[0].ID != "vid_01" || res.Tracks[29].ID != "vid_30" {
		t.Fatalf("unexpected boundary track IDs: first=%s last=%s", res.Tracks[0].ID, res.Tracks[29].ID)
	}
	if res.ContinuationToken != "CONT_PAGE_3" {
		t.Fatalf("expected CONT_PAGE_3, got %q", res.ContinuationToken)
	}

	// Verify overflow tracks (31..35) can be persisted and restored across restarts.
	savedOverflow := client.GetOverflow()
	if len(savedOverflow) != 5 || savedOverflow[0].ID != "vid_31" {
		t.Fatalf("expected 5 overflow tracks starting with vid_31, got %+v", savedOverflow)
	}
	restoredClient := NewClient(srv.Client())
	restoredClient.SetBaseURL(srv.URL)
	restoredClient.SetOverflow(savedOverflow)

	existing := make(map[string]struct{}, len(res.Tracks))
	for _, tr := range res.Tracks {
		existing[tr.ID] = struct{}{}
	}

	// First LoadMore on restored client drains restored overflow tracks (31..35) and tops up from CONT_PAGE_3 (36..50).
	more1, err := restoredClient.FetchMoreSongs(context.Background(), res.ContinuationToken, "vid_30", existing)
	if err != nil {
		t.Fatalf("FetchMoreSongs(1) failed: %v", err)
	}
	if len(more1.Tracks) != 20 {
		t.Fatalf("expected 20 tracks (5 overflow + 15 page 3), got %d", len(more1.Tracks))
	}
	if len(restoredClient.GetOverflow()) != 0 {
		t.Fatalf("expected overflow drained after FetchMoreSongs")
	}
	for _, tr := range more1.Tracks {
		existing[tr.ID] = struct{}{}
	}

	// Second LoadMore fetches CONT_PAGE_4 (51..60).
	more2, err := restoredClient.FetchMoreSongs(context.Background(), more1.ContinuationToken, "vid_50", existing)
	if err != nil {
		t.Fatalf("FetchMoreSongs(2) failed: %v", err)
	}
	if len(more2.Tracks) != 10 {
		t.Fatalf("expected 10 tracks from CONT_PAGE_4, got %d", len(more2.Tracks))
	}
}

func TestLiveYouTubeSearch30Songs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live YouTube test in -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client := NewClient(nil)
	res, err := client.SearchInitialPlaylist(ctx, "daft punk", 30)
	if err != nil {
		t.Fatalf("live search failed: %v", err)
	}
	if len(res.Tracks) < 25 {
		t.Fatalf("expected ~30 tracks from live search, got %d", len(res.Tracks))
	}
	t.Logf("live search returned %d tracks, first=%q (%s), contLen=%d",
		len(res.Tracks), res.Tracks[0].Title, res.Tracks[0].DurationText, len(res.ContinuationToken))
}
