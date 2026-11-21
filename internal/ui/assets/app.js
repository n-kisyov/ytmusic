(() => {
  const media = document.getElementById('media-player');
  const videoPane = document.getElementById('video-pane');
  const videoLoading = document.getElementById('video-loading-overlay');
  const searchForm = document.getElementById('search-form');
  const searchInput = document.getElementById('search-input');
  const searchDropdown = document.getElementById('search-dropdown');
  const searchBtnIcon = document.getElementById('search-btn-icon');
  const searchSpinner = document.getElementById('search-spinner');

  const themeBtn = document.getElementById('theme-btn');
  const sunIcon = document.getElementById('sun-icon');
  const moonIcon = document.getElementById('moon-icon');
  const pinBtn = document.getElementById('pin-btn');
  const authBtn = document.getElementById('auth-btn');
  const authAvatarImg = document.getElementById('auth-avatar-img');
  const authDefaultIcon = document.getElementById('auth-default-icon');
  const authPopover = document.getElementById('auth-popover');
  const authPopoverAvatar = document.getElementById('auth-popover-avatar');
  const authPopoverInitial = document.getElementById('auth-popover-initial');
  const authAccountName = document.getElementById('auth-account-name');
  const authAccountMeta = document.getElementById('auth-account-meta');
  const authReauthBanner = document.getElementById('auth-reauth-banner');
  const authReauthBtn = document.getElementById('auth-reauth-btn');
  const authAccountsSection = document.getElementById('auth-accounts-section');
  const authRefreshBtn = document.getElementById('auth-refresh-btn');
  const authAccountsList = document.getElementById('auth-accounts-list');
  const authSwitchGoogleBtn = document.getElementById('auth-switch-google-btn');
  const authSignoutBtn = document.getElementById('auth-signout-btn');
  const authCloseBtn = document.getElementById('auth-close-btn');
  const quitBtn = document.getElementById('quit-btn');

  const currentThumb = document.getElementById('current-thumb');
  const thumbPlaceholder = document.getElementById('thumb-placeholder');
  const currentTitle = document.getElementById('current-title');
  const currentArtist = document.getElementById('current-artist');
  const likeBtn = document.getElementById('like-btn');
  const heartOutlineIcon = document.getElementById('heart-outline-icon');
  const heartFilledIcon = document.getElementById('heart-filled-icon');
  const trackCounter = document.getElementById('track-counter');
  const vuMeter = document.getElementById('vu-meter');
  const vuCanvas = document.getElementById('vu-canvas');
  const playtimeStat = document.getElementById('playtime-stat');

  const timeElapsed = document.getElementById('time-elapsed');
  const timeTotal = document.getElementById('time-total');
  const progressRange = document.getElementById('progress-range');
  const progressFill = document.getElementById('progress-fill');
  const progressBuffer = document.getElementById('progress-buffer');

  const shuffleBtn = document.getElementById('shuffle-btn');
  const prevBtn = document.getElementById('prev-btn');
  const playBtn = document.getElementById('play-btn');
  const nextBtn = document.getElementById('next-btn');
  const repeatBtn = document.getElementById('repeat-btn');
  const repeatAllIcon = document.getElementById('repeat-all-icon');
  const repeatOneIcon = document.getElementById('repeat-one-icon');
  const playIcon = document.getElementById('play-icon');
  const pauseIcon = document.getElementById('pause-icon');

  const muteBtn = document.getElementById('mute-btn');
  const volIcon = document.getElementById('vol-icon');
  const muteIcon = document.getElementById('mute-icon');
  const volumeRange = document.getElementById('volume-range');

  const eqBtn = document.getElementById('eq-btn');
  const eqBtnLabel = document.getElementById('eq-btn-label');
  const videoToggleBtn = document.getElementById('video-toggle-btn');
  const playlistToggleBtn = document.getElementById('playlist-toggle-btn');

  const audioPopover = document.getElementById('audio-popover');
  const audioPopClose = document.getElementById('audio-pop-close');
  const shortcutsHelpBtn = document.getElementById('shortcuts-help-btn');
  const sleepCountdownLabel = document.getElementById('sleep-countdown-label');
  const curCopyLinkBtn = document.getElementById('cur-copy-link-btn');
  const curOpenYtBtn = document.getElementById('cur-open-yt-btn');
  const curRadioBtn = document.getElementById('cur-radio-btn');
  const shortcutsPopover = document.getElementById('shortcuts-popover');
  const shortcutsCloseBtn = document.getElementById('shortcuts-close-btn');

  const playlistDrawer = document.getElementById('playlist-drawer');
  const playlistScroll = document.getElementById('playlist-scroll');
  const playlistList = document.getElementById('playlist-list');
  const playlistEmpty = document.getElementById('playlist-empty');
  const loadMoreFooter = document.getElementById('load-more-footer');
  const statusToast = document.getElementById('status-toast');

  const tabQueue = document.getElementById('tab-queue');
  const tabLiked = document.getElementById('tab-liked');
  const tabSaved = document.getElementById('tab-saved');
  const drawerTabsScroll = document.getElementById('drawer-tabs-scroll');
  const customPlaylistTabs = document.getElementById('custom-playlist-tabs');
  const saveQueueBtn = document.getElementById('save-queue-btn');
  const saveQueueBar = document.getElementById('save-queue-bar');
  const saveQueueInput = document.getElementById('save-queue-input');
  const saveQueueConfirm = document.getElementById('save-queue-confirm');
  const saveQueueMerge = document.getElementById('save-queue-merge');
  const saveEmptyConfirm = document.getElementById('save-empty-confirm');
  const saveQueueCancel = document.getElementById('save-queue-cancel');
  const filterRow = document.getElementById('filter-row');
  const playlistFilter = document.getElementById('playlist-filter');
  const clearQueueBtn = document.getElementById('clear-queue-btn');
  const appendCollectionBtn = document.getElementById('append-collection-btn');
  const playLikedBtn = document.getElementById('play-liked-btn');
  const addPlaylistPopover = document.getElementById('add-playlist-popover');
  const addPlaylistOpenBtn = document.getElementById('add-playlist-open-btn');
  const addPlaylistCopyBtn = document.getElementById('add-playlist-copy-btn');
  const addPlaylistClose = document.getElementById('add-playlist-close');
  const addPlaylistTrack = document.getElementById('add-playlist-track');
  const addPlaylistList = document.getElementById('add-playlist-list');
  const addPlaylistForm = document.getElementById('add-playlist-form');
  const addPlaylistInput = document.getElementById('add-playlist-input');

  const state = {
    playlist: [],
    likedTracks: [],
    savedPlaylists: [],
    currentIndex: -1,
    videoEnabled: false,
    playlistExpanded: true,
    alwaysOnTop: false,
    theme: 'dark',
    shuffle: false,
    repeatMode: 'off', // 'off' | 'all' | 'one'
    shuffleHistory: [],
    shuffleBag: [],
    activeTab: 'queue', // 'queue' | 'liked' | 'saved'
    filterQuery: '',
    volume: 0.8,
    muted: false,
    isSeeking: false,
    isSearching: false,
    isLoadingMore: false,
    pendingSeekSec: 0,
    playbackSeq: 0,
    consecutiveErrors: 0,
    errorSkipTimer: null,
    lastSyncMs: 0,
    loggedIn: false,
    accountName: '',
    channelHandle: '',
    email: '',
    avatarUrl: '',
    activePageId: '',
    authUser: '',
    accounts: [],
    needsReauth: false,
    toastTimer: null,
    vuStyle: 'led', // 'led' | 'bar'
    eqPreset: 'flat', // 'flat' | 'bass' | 'vocal' | 'bright'
    playbackRate: 1.0,
    recentSearches: [],
    sleepEndMs: 0,
    sleepTickTimer: null,
    searchSuggestTimer: null,
    dragSourceIdx: -1,
    userInitiatedPlay: false,
    baseTotalPlaySec: 0,
    sessionPlayMs: 0,
    playAnchorMs: 0,
    playtimeTimer: null,
    popoverTrack: null,
  };

  function formatCompactPlayTime(sec) {
    if (!Number.isFinite(sec) || sec < 0) sec = 0;
    const total = Math.floor(sec);
    const hrs = Math.floor(total / 3600);
    const mins = Math.floor((total % 3600) / 60);
    const secs = total % 60;
    if (hrs > 0) {
      return `${hrs}h${mins < 10 ? '0' : ''}${mins}m`;
    }
    return `${mins}:${secs < 10 ? '0' : ''}${secs}`;
  }

  function formatFullPlayTime(sec) {
    if (!Number.isFinite(sec) || sec < 0) sec = 0;
    const total = Math.floor(sec);
    const hrs = Math.floor(total / 3600);
    const mins = Math.floor((total % 3600) / 60);
    const secs = total % 60;
    const pad = (n) => (n < 10 ? '0' + n : '' + n);
    if (hrs > 0) return `${hrs}h ${pad(mins)}m ${pad(secs)}s`;
    return `${mins}m ${pad(secs)}s`;
  }

  function updatePlaytimeUI() {
    if (!playtimeStat) return;
    const sessSec = Math.floor(state.sessionPlayMs / 1000);
    const totalSec = state.baseTotalPlaySec + sessSec;
    playtimeStat.textContent = `${formatCompactPlayTime(sessSec)} / ${formatCompactPlayTime(totalSec)}`;
    playtimeStat.title = `Session: ${formatFullPlayTime(sessSec)} • All-time: ${formatFullPlayTime(totalSec)}`;
  }

  function openTrackExternal(tr) {
    if (!tr || !tr.id) return;
    const url = `https://www.youtube.com/watch?v=${tr.id}`;
    if (typeof window.backendOpenExternal === 'function') {
      window.backendOpenExternal(url).catch(() => showToast('Could not open browser'));
    }
  }

  function flushSessionPlayTime() {
    if (state.playAnchorMs > 0) {
      const now = performance.now();
      if (!media.paused && !media.ended && media.readyState >= 2) {
        const delta = now - state.playAnchorMs;
        if (delta > 0 && delta < 3600000) {
          state.sessionPlayMs += delta;
        }
      }
      state.playAnchorMs = now;
    }
  }

  function startPlaytimeTracker() {
    flushSessionPlayTime();
    state.playAnchorMs = performance.now();
    if (!state.playtimeTimer) {
      state.playtimeTimer = setInterval(() => {
        flushSessionPlayTime();
        updatePlaytimeUI();
      }, 1000);
    }
  }

  function stopPlaytimeTracker() {
    flushSessionPlayTime();
    state.playAnchorMs = 0;
    if (state.playtimeTimer) {
      clearInterval(state.playtimeTimer);
      state.playtimeTimer = null;
    }
    updatePlaytimeUI();
  }

  function copyTrackURL(tr) {
    if (!tr || !tr.id) return;
    const url = `https://www.youtube.com/watch?v=${tr.id}`;
    if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
      navigator.clipboard.writeText(url).then(
        () => showToast('Copied YouTube link'),
        () => showToast('Clipboard unavailable')
      );
    } else {
      showToast(url);
    }
  }

  function showToast(msg) {
    statusToast.textContent = msg;
    statusToast.classList.remove('hidden');
    if (state.toastTimer) clearTimeout(state.toastTimer);
    state.toastTimer = setTimeout(() => {
      statusToast.classList.add('hidden');
    }, 2600);
  }
  window.__showToast = showToast;

  function formatTime(sec) {
    if (!Number.isFinite(sec) || sec < 0) return '0:00';
    const total = Math.floor(sec);
    const hrs = Math.floor(total / 3600);
    const mins = Math.floor((total % 3600) / 60);
    const secs = total % 60;
    const pad = (n) => (n < 10 ? '0' + n : '' + n);
    if (hrs > 0) return `${hrs}:${pad(mins)}:${pad(secs)}`;
    return `${mins}:${pad(secs)}`;
  }

  function isTrackLiked(id) {
    return Boolean(id) && state.likedTracks.some((t) => t.id === id);
  }

  function updateLikeButtonUI() {
    const cur = state.currentIndex >= 0 && state.currentIndex < state.playlist.length
      ? state.playlist[state.currentIndex]
      : null;
    const liked = cur ? isTrackLiked(cur.id) : false;
    likeBtn.classList.toggle('liked', liked);
    heartOutlineIcon.classList.toggle('hidden', liked);
    heartFilledIcon.classList.toggle('hidden', !liked);
    likeBtn.title = liked ? 'Remove from Liked Songs' : 'Add to Liked Songs';
  }

  function updateModesUI() {
    shuffleBtn.classList.toggle('active', state.shuffle);
    shuffleBtn.title = state.shuffle ? 'Shuffle: On (S)' : 'Shuffle: Off (S)';

    repeatBtn.classList.toggle('active', state.repeatMode !== 'off');
    repeatAllIcon.classList.toggle('hidden', state.repeatMode === 'one');
    repeatOneIcon.classList.toggle('hidden', state.repeatMode !== 'one');
    const rLabel = state.repeatMode === 'one' ? 'Repeat: One (R)' : state.repeatMode === 'all' ? 'Repeat: All (R)' : 'Repeat: Off (R)';
    repeatBtn.title = rLabel;
  }

  function updatePlayButtonUI() {
    const playing = !media.paused && !media.ended;
    playIcon.classList.toggle('hidden', playing);
    pauseIcon.classList.toggle('hidden', !playing);
    if ('mediaSession' in navigator) {
      navigator.mediaSession.playbackState = playing ? 'playing' : 'paused';
    }
  }

  function updateMediaSessionMetadata(tr) {
    if (!('mediaSession' in navigator) || typeof MediaMetadata === 'undefined') return;
    if (!tr) {
      navigator.mediaSession.metadata = null;
      return;
    }
    navigator.mediaSession.metadata = new MediaMetadata({
      title: tr.title || 'ytmusic',
      artist: tr.artist || 'YouTube',
      artwork: tr.thumbnail ? [{ src: tr.thumbnail, sizes: '480x360', type: 'image/jpeg' }] : [],
    });
  }

  function updateVolumeUI() {
    const muted = state.muted || state.volume <= 0.001;
    volIcon.classList.toggle('hidden', muted);
    muteIcon.classList.toggle('hidden', !muted);
    volumeRange.value = String(state.volume);
    media.volume = state.volume;
    media.muted = state.muted;
  }

  function applyVideoModeUI(enabled) {
    state.videoEnabled = enabled;
    videoPane.classList.toggle('open', enabled);
    videoToggleBtn.classList.toggle('active', enabled);
  }

  function applyPlaylistModeUI(expanded) {
    state.playlistExpanded = expanded;
    playlistDrawer.classList.toggle('open', expanded);
    playlistToggleBtn.classList.toggle('active', expanded);
    if (!expanded) {
      closeAddToPlaylistPopover();
    }
  }

  function applyAlwaysOnTopUI(enabled) {
    state.alwaysOnTop = enabled;
    pinBtn.classList.toggle('active', enabled);
    pinBtn.title = enabled ? 'Stay on top: On' : 'Stay on top: Off';
  }

  function applyThemeUI(theme) {
    const normalized = theme === 'light' ? 'light' : 'dark';
    state.theme = normalized;
    document.documentElement.setAttribute('data-theme', normalized);
    const isLight = normalized === 'light';
    sunIcon.classList.toggle('hidden', isLight);
    moonIcon.classList.toggle('hidden', !isLight);
    themeBtn.title = isLight ? 'Switch to Dark theme (T)' : 'Switch to Light theme (T)';
    drawVuMeter();
  }

  function setAvatarDisplay(imgEl, fallbackEl, url, fallbackName) {
    if (fallbackEl && fallbackName && fallbackEl.tagName && fallbackEl.tagName.toLowerCase() !== 'svg') {
      fallbackEl.textContent = fallbackName.trim().charAt(0).toUpperCase() || 'Y';
    }
    if (!imgEl) return;
    if (url) {
      imgEl.onload = () => {
        imgEl.classList.remove('hidden');
        if (fallbackEl) fallbackEl.classList.add('hidden');
      };
      imgEl.onerror = () => {
        imgEl.classList.add('hidden');
        if (fallbackEl) fallbackEl.classList.remove('hidden');
      };
      imgEl.src = url;
    } else {
      imgEl.removeAttribute('src');
      imgEl.classList.add('hidden');
      if (fallbackEl) fallbackEl.classList.remove('hidden');
    }
  }

  function renderAuthAccountsList() {
    if (!authAccountsList || !authAccountsSection) return;
    const accounts = Array.isArray(state.accounts) ? state.accounts : [];
    authAccountsSection.classList.toggle('hidden', accounts.length === 0);
    authAccountsList.innerHTML = '';
    if (accounts.length === 0) return;

    const hasPageMatch = accounts.some((a) => (a.page_id || '') === (state.activePageId || ''));
    const frag = document.createDocumentFragment();
    accounts.forEach((acc) => {
      const btn = document.createElement('button');
      btn.type = 'button';
      const isSelected = hasPageMatch
        ? ((acc.page_id || '') === (state.activePageId || ''))
        : Boolean(acc.selected);
      btn.className = 'auth-account-item' + (isSelected ? ' active' : '');

      const avWrap = document.createElement('span');
      avWrap.className = 'auth-item-avatar';
      const avInitial = document.createElement('span');
      avInitial.textContent = (acc.name || 'Y').trim().charAt(0).toUpperCase() || 'Y';
      const avImg = document.createElement('img');
      avImg.className = 'hidden';
      avImg.alt = '';
      setAvatarDisplay(avImg, avInitial, acc.avatar_url || '', acc.name || 'Y');
      avWrap.appendChild(avImg);
      avWrap.appendChild(avInitial);

      const meta = document.createElement('span');
      meta.className = 'auth-item-meta';
      const nameEl = document.createElement('span');
      nameEl.className = 'auth-item-name';
      nameEl.textContent = acc.name || 'YouTube Channel';
      const subEl = document.createElement('span');
      subEl.className = 'auth-item-sub';
      const subParts = [acc.handle, acc.subscriber_text, acc.email].filter(Boolean);
      subEl.textContent = subParts.join(' • ') || (acc.page_id ? 'Brand Account' : 'Personal Account');
      meta.appendChild(nameEl);
      meta.appendChild(subEl);

      btn.appendChild(avWrap);
      btn.appendChild(meta);

      if (isSelected) {
        const chk = document.createElement('span');
        chk.className = 'auth-item-check';
        chk.textContent = '✓';
        btn.appendChild(chk);
      }

      btn.addEventListener('click', async () => {
        if (isSelected) return;
        try {
          const updated = await window.backendSwitchBrandAccount(acc.page_id || '', acc.auth_user || '');
          applyAuthUI(updated);
          showToast(`Switched to ${updated.account_name || acc.name}`);
        } catch (_) {
          showToast('Failed to switch channel');
        }
      });

      frag.appendChild(btn);
    });
    authAccountsList.appendChild(frag);
  }

  function applyAuthUI(auth) {
    state.loggedIn = Boolean(auth && auth.logged_in);
    state.needsReauth = Boolean(auth && auth.needs_reauth);
    state.accountName = (auth && auth.account_name) || '';
    state.channelHandle = (auth && auth.channel_handle) || '';
    state.email = (auth && auth.email) || '';
    state.avatarUrl = (auth && auth.avatar_url) || '';
    state.activePageId = (auth && auth.active_page_id) || '';
    state.authUser = (auth && auth.auth_user) || '';
    state.accounts = Array.isArray(auth && auth.accounts) ? auth.accounts : [];

    authBtn.classList.toggle('signed-in', state.loggedIn && !state.needsReauth);
    authBtn.classList.toggle('needs-reauth', state.loggedIn && state.needsReauth);

    const displayName = state.needsReauth && (!state.accountName || state.accountName === 'YouTube Account')
      ? 'Google Sign-In'
      : (state.accountName || state.email || 'Google Account');

    const metaParts = [state.channelHandle, state.email].filter(Boolean);
    const metaText = metaParts.join(' • ') || (state.needsReauth ? 'Session refresh required' : 'Signed in');

    authBtn.title = state.loggedIn
      ? (state.needsReauth ? 'Refresh YouTube sign-in' : `Signed in as ${displayName}`)
      : 'Sign in with Google';

    setAvatarDisplay(authAvatarImg, authDefaultIcon, state.loggedIn ? state.avatarUrl : '', displayName);
    setAvatarDisplay(authPopoverAvatar, authPopoverInitial, state.loggedIn ? state.avatarUrl : '', displayName);

    authAccountName.textContent = displayName;
    if (authAccountMeta) {
      authAccountMeta.textContent = metaText;
    }
    if (authReauthBanner) {
      authReauthBanner.classList.toggle('hidden', !state.needsReauth);
    }
    renderAuthAccountsList();

    if (!state.loggedIn) {
      authPopover.classList.add('hidden');
    }
  }

  function updateNowPlayingHeader() {
    const total = state.playlist.length;
    if (state.currentIndex >= 0 && state.currentIndex < total) {
      const tr = state.playlist[state.currentIndex];
      currentTitle.textContent = tr.title || 'Unknown Title';
      currentArtist.textContent = tr.artist || 'YouTube';
      trackCounter.textContent = `${state.currentIndex + 1}/${total}`;
      if (tr.thumbnail) {
        currentThumb.src = tr.thumbnail;
        currentThumb.classList.remove('hidden');
        thumbPlaceholder.classList.add('hidden');
      } else {
        currentThumb.removeAttribute('src');
        currentThumb.classList.add('hidden');
        thumbPlaceholder.classList.remove('hidden');
      }
      const dur = Number.isFinite(media.duration) && media.duration > 0 ? media.duration : (tr.duration_sec || 0);
      timeTotal.textContent = formatTime(dur);
      updateMediaSessionMetadata(tr);
    } else {
      currentTitle.textContent = 'Search for a song to start';
      currentArtist.textContent = state.videoEnabled ? 'Video playback on' : 'Audio-only mode';
      trackCounter.textContent = `0/${total}`;
      currentThumb.classList.add('hidden');
      thumbPlaceholder.classList.remove('hidden');
      timeTotal.textContent = '0:00';
      timeElapsed.textContent = '0:00';
      progressFill.style.width = '0%';
      progressBuffer.style.width = '0%';
      progressRange.value = '0';
      updateMediaSessionMetadata(null);
    }
    updateLikeButtonUI();
  }

  // --- Minimalistic Stereo (L/R) VU Meter & 3-Band Web Audio EQ ---
  const EQ_PRESETS = {
    flat: [0, 0, 0],
    bass: [6, 0, 1],
    vocal: [-2, 4.5, 2],
    bright: [0, 1.5, 5.5],
  };

  const vu = {
    audioCtx: null,
    sourceNode: null,
    eqLow: null,
    eqMid: null,
    eqHigh: null,
    splitter: null,
    analyserL: null,
    analyserR: null,
    bufL: null,
    bufR: null,
    graphAttached: false,
    attachInFlight: false,
    rafId: 0,
    lastTs: 0,
    levelL: 0,
    levelR: 0,
    peakL: 0,
    peakR: 0,
    holdL: 0,
    holdR: 0,
    ctx2d: vuCanvas ? vuCanvas.getContext('2d') : null,
  };

  function applyEQPresetGains() {
    if (!vu.eqLow || !vu.eqMid || !vu.eqHigh) return;
    const [gLow, gMid, gHigh] = EQ_PRESETS[state.eqPreset] || EQ_PRESETS.flat;
    vu.eqLow.gain.value = gLow;
    vu.eqMid.gain.value = gMid;
    vu.eqHigh.gain.value = gHigh;
  }

  function attachStereoAnalyserNodes() {
    if (vu.graphAttached || !vu.audioCtx || vu.audioCtx.state !== 'running') return;
    try {
      vu.sourceNode = vu.audioCtx.createMediaElementSource(media);
      vu.eqLow = vu.audioCtx.createBiquadFilter();
      vu.eqLow.type = 'lowshelf';
      vu.eqLow.frequency.value = 100;

      vu.eqMid = vu.audioCtx.createBiquadFilter();
      vu.eqMid.type = 'peaking';
      vu.eqMid.frequency.value = 1000;
      vu.eqMid.Q.value = 1.0;

      vu.eqHigh = vu.audioCtx.createBiquadFilter();
      vu.eqHigh.type = 'highshelf';
      vu.eqHigh.frequency.value = 8000;
      applyEQPresetGains();

      vu.splitter = vu.audioCtx.createChannelSplitter(2);
      vu.analyserL = vu.audioCtx.createAnalyser();
      vu.analyserR = vu.audioCtx.createAnalyser();
      vu.analyserL.fftSize = 512;
      vu.analyserR.fftSize = 512;
      vu.analyserL.smoothingTimeConstant = 0.5;
      vu.analyserR.smoothingTimeConstant = 0.5;
      vu.bufL = new Float32Array(vu.analyserL.fftSize);
      vu.bufR = new Float32Array(vu.analyserR.fftSize);

      vu.sourceNode.connect(vu.eqLow);
      vu.eqLow.connect(vu.eqMid);
      vu.eqMid.connect(vu.eqHigh);
      vu.eqHigh.connect(vu.audioCtx.destination);
      vu.eqHigh.connect(vu.splitter);
      vu.splitter.connect(vu.analyserL, 0);
      vu.splitter.connect(vu.analyserR, 1);
      vu.graphAttached = true;
      removeAudioUnlockListeners();
    } catch (_) {
      // Ignore if MediaElementSource cannot attach cleanly
    }
  }

  function ensureAudioAnalyser() {
    if (vu.graphAttached) return;
    const AudioCtx = window.AudioContext || window.webkitAudioContext;
    if (!AudioCtx) return;

    if (!vu.audioCtx) {
      try {
        vu.audioCtx = new AudioCtx();
      } catch (_) {
        return;
      }
    }

    if (vu.audioCtx.state === 'running') {
      attachStereoAnalyserNodes();
      return;
    }

    if (!vu.attachInFlight) {
      vu.attachInFlight = true;
      vu.audioCtx.resume().then(() => {
        vu.attachInFlight = false;
        attachStereoAnalyserNodes();
      }).catch(() => {
        vu.attachInFlight = false;
      });
    }
  }

  function measureChannelLevel(analyser, buf) {
    analyser.getFloatTimeDomainData(buf);
    let sumSq = 0;
    let peak = 0;
    for (let i = 0; i < buf.length; i++) {
      const s = buf[i];
      sumSq += s * s;
      const abs = s < 0 ? -s : s;
      if (abs > peak) peak = abs;
    }
    const rms = Math.sqrt(sumSq / buf.length);
    let amp = Math.max(rms * 1.45, peak * 0.85);
    // Compensate for HTMLMediaElement volume attenuation so VU remains legible at low volume.
    if (!state.muted && state.volume > 0.01 && state.volume < 0.99) {
      amp = amp / Math.max(0.18, state.volume);
    }
    if (amp <= 0.0001) return 0;
    const db = 20 * Math.log10(amp);
    return Math.max(0, Math.min(1, (db + 38) / 37));
  }

  function getVuPalette() {
    const isLight = state.theme === 'light';
    return {
      unlit: isLight ? 'rgba(0, 0, 0, 0.12)' : 'rgba(255, 255, 255, 0.12)',
      green: isLight ? '#0f7b0f' : '#2ba640',
      amber: isLight ? '#d97706' : '#f5a623',
      red: isLight ? '#e60000' : '#ff0000',
    };
  }

  function vuZoneColor(norm, palette) {
    if (norm >= 0.8) return palette.red;
    if (norm >= 0.62) return palette.amber;
    return palette.green;
  }

  function drawVuChannel(ctx, y, h, level, peak, palette, grad) {
    const width = 44;
    if (state.vuStyle === 'bar') {
      ctx.fillStyle = palette.unlit;
      ctx.fillRect(0, y, width, h);
      const fillW = Math.max(0, Math.min(width, level * width));
      if (fillW > 0.5) {
        ctx.fillStyle = grad;
        ctx.fillRect(0, y, fillW, h);
      }
      if (peak > 0.02) {
        const peakX = Math.max(0, Math.min(width - 1.5, peak * width - 1.5));
        ctx.fillStyle = vuZoneColor(peak, palette);
        ctx.fillRect(peakX, y, 1.5, h);
      }
      return;
    }

    const segments = 11;
    const segW = 3;
    const gap = 1;
    const litCount = level > 0.02 ? Math.max(1, Math.round(level * segments)) : 0;
    const peakIdx = peak > 0.02 ? Math.min(segments - 1, Math.max(0, Math.ceil(peak * segments) - 1)) : -1;

    for (let i = 0; i < segments; i++) {
      const x = i * (segW + gap);
      const active = i < litCount || i === peakIdx;
      if (!active) {
        ctx.fillStyle = palette.unlit;
      } else if (i >= 9) {
        ctx.fillStyle = palette.red;
      } else if (i >= 7) {
        ctx.fillStyle = palette.amber;
      } else {
        ctx.fillStyle = palette.green;
      }
      ctx.fillRect(x, y, segW, h);
    }
  }

  function drawVuMeter() {
    if (!vuCanvas || !vu.ctx2d) return;
    const cssW = 44;
    const cssH = 14;
    const dpr = Math.min(3, window.devicePixelRatio || 1);
    const targetW = Math.round(cssW * dpr);
    const targetH = Math.round(cssH * dpr);
    if (vuCanvas.width !== targetW || vuCanvas.height !== targetH) {
      vuCanvas.width = targetW;
      vuCanvas.height = targetH;
    }

    const ctx = vu.ctx2d;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, cssW, cssH);

    const palette = getVuPalette();
    let grad = null;
    if (state.vuStyle === 'bar') {
      grad = ctx.createLinearGradient(0, 0, cssW, 0);
      grad.addColorStop(0, palette.green);
      grad.addColorStop(0.65, palette.green);
      grad.addColorStop(0.82, palette.amber);
      grad.addColorStop(1, palette.red);
    }

    drawVuChannel(ctx, 1, 5, vu.levelL, vu.peakL, palette, grad);
    drawVuChannel(ctx, 8, 5, vu.levelR, vu.peakR, palette, grad);
  }

  function updateChannelBallistics(levelKey, peakKey, holdKey, target, step) {
    const cur = vu[levelKey];
    if (target > cur) {
      vu[levelKey] = cur + (target - cur) * Math.min(1, 0.55 * step);
    } else {
      vu[levelKey] = Math.max(target, cur - 0.045 * step);
    }

    if (vu[levelKey] >= vu[peakKey]) {
      vu[peakKey] = vu[levelKey];
      vu[holdKey] = 20;
    } else if (vu[holdKey] > 0) {
      vu[holdKey] = Math.max(0, vu[holdKey] - step);
    } else {
      vu[peakKey] = Math.max(0, vu[peakKey] - 0.018 * step);
    }
  }

  function tickVuMeter(ts) {
    const dtMs = vu.lastTs > 0 ? Math.min(50, Math.max(4, ts - vu.lastTs)) : 16.6667;
    vu.lastTs = ts;
    const step = dtMs / 16.6667;

    const playing = !media.paused && !media.ended && media.readyState >= 2;
    const audible = playing && !state.muted && state.volume > 0.001;

    let targetL = 0;
    let targetR = 0;
    if (audible) {
      if (vu.graphAttached && vu.analyserL && vu.analyserR) {
        targetL = measureChannelLevel(vu.analyserL, vu.bufL);
        targetR = measureChannelLevel(vu.analyserR, vu.bufR);
      } else {
        // Gentle idle pulse only while AudioContext awaits first user gesture during autoplay
        const phase = ts * 0.0045;
        targetL = Math.max(0.08, Math.min(0.45, (0.24 + 0.14 * Math.sin(phase) + 0.06 * Math.sin(phase * 2.3)) * state.volume));
        targetR = Math.max(0.08, Math.min(0.45, (0.24 + 0.14 * Math.sin(phase + 0.9) + 0.06 * Math.cos(phase * 1.9)) * state.volume));
      }
    }

    updateChannelBallistics('levelL', 'peakL', 'holdL', targetL, step);
    updateChannelBallistics('levelR', 'peakR', 'holdR', targetR, step);
    drawVuMeter();

    if (playing || vu.levelL > 0.002 || vu.levelR > 0.002 || vu.peakL > 0.002 || vu.peakR > 0.002) {
      vu.rafId = requestAnimationFrame(tickVuMeter);
    } else {
      vu.rafId = 0;
      vu.lastTs = 0;
    }
  }

  function startVuMeterLoop() {
    ensureAudioAnalyser();
    if (!vu.rafId) {
      vu.lastTs = 0;
      vu.rafId = requestAnimationFrame(tickVuMeter);
    }
  }

  function decayVuMeterToIdle() {
    if (!vu.rafId && (vu.levelL > 0 || vu.levelR > 0 || vu.peakL > 0 || vu.peakR > 0)) {
      vu.lastTs = 0;
      vu.rafId = requestAnimationFrame(tickVuMeter);
    }
  }

  function unlockAudioContextOnGesture() {
    ensureAudioAnalyser();
  }

  function removeAudioUnlockListeners() {
    document.removeEventListener('pointerdown', unlockAudioContextOnGesture);
    document.removeEventListener('keydown', unlockAudioContextOnGesture);
  }

  drawVuMeter();

  const CUSTOM_TAB_PREFIX = 'playlist:';

  function getSelectedSavedPlaylistName() {
    return state.activeTab.startsWith(CUSTOM_TAB_PREFIX)
      ? state.activeTab.slice(CUSTOM_TAB_PREFIX.length)
      : '';
  }

  function findSavedPlaylist(name) {
    if (!name) return null;
    const lower = name.toLowerCase();
    return state.savedPlaylists.find((sp) => (sp.name || '').toLowerCase() === lower) || null;
  }

  function playlistContainsTrack(sp, trackID) {
    return Boolean(trackID) && Array.isArray(sp?.tracks) && sp.tracks.some((t) => t.id === trackID);
  }

  function updateTabsUI() {
    const customName = getSelectedSavedPlaylistName();
    const selectedSp = customName ? findSavedPlaylist(customName) : null;
    if (customName && !selectedSp) {
      state.activeTab = 'queue';
    }

    tabQueue.classList.toggle('active', state.activeTab === 'queue');
    tabLiked.classList.toggle('active', state.activeTab === 'liked');
    tabSaved.classList.toggle('active', state.activeTab === 'saved');

    tabQueue.textContent = state.playlist.length > 0 ? `Queue (${state.playlist.length})` : 'Queue';
    tabLiked.textContent = state.likedTracks.length > 0 ? `Liked (${state.likedTracks.length})` : 'Liked ♥';
    tabSaved.textContent = state.savedPlaylists.length > 0 ? `Saved (${state.savedPlaylists.length})` : 'Saved';

    if (customPlaylistTabs) {
      customPlaylistTabs.innerHTML = '';
      const frag = document.createDocumentFragment();
      state.savedPlaylists.forEach((sp) => {
        const count = Array.isArray(sp.tracks) ? sp.tracks.length : 0;
        const pill = document.createElement('button');
        pill.type = 'button';
        pill.className = 'tab-pill' + (selectedSp && sp.name.toLowerCase() === selectedSp.name.toLowerCase() ? ' active' : '');
        pill.textContent = `${sp.name} (${count})`;
        pill.title = `${sp.name} (${count} song${count === 1 ? '' : 's'})`;
        pill.addEventListener('click', () => {
          closeAddToPlaylistPopover();
          state.activeTab = CUSTOM_TAB_PREFIX + sp.name;
          renderActiveDrawerView(false);
          const activePill = customPlaylistTabs.querySelector('.tab-pill.active');
          if (activePill) {
            activePill.scrollIntoView({ block: 'nearest', inline: 'nearest' });
          }
        });
        frag.appendChild(pill);
      });
      customPlaylistTabs.appendChild(frag);
    }

    filterRow.classList.toggle('hidden', state.activeTab === 'saved');
    const canPlayCollection =
      (state.activeTab === 'liked' && state.likedTracks.length > 0) ||
      Boolean(selectedSp && Array.isArray(selectedSp.tracks) && selectedSp.tracks.length > 0);
    playLikedBtn.classList.toggle('hidden', !canPlayCollection);
    if (clearQueueBtn) {
      clearQueueBtn.classList.toggle('hidden', state.activeTab !== 'queue' || state.playlist.length === 0);
    }
    if (appendCollectionBtn) {
      appendCollectionBtn.classList.toggle('hidden', !canPlayCollection);
    }
    saveQueueBtn.classList.remove('hidden');
    if (saveQueueConfirm) {
      saveQueueConfirm.classList.toggle('hidden', state.playlist.length === 0);
    }
    updateSaveBarButtonsUI();
  }

  function updateSaveBarButtonsUI() {
    if (!saveQueueConfirm || !saveQueueMerge || !saveQueueInput) return;
    if (state.playlist.length === 0) {
      saveQueueConfirm.classList.add('hidden');
      saveQueueMerge.classList.add('hidden');
      return;
    }
    const name = saveQueueInput.value.trim();
    const exists = Boolean(name && findSavedPlaylist(name));
    saveQueueConfirm.classList.remove('hidden');
    saveQueueConfirm.textContent = exists ? 'Replace' : 'Save Queue';
    saveQueueMerge.classList.toggle('hidden', !exists);
  }

  function matchesFilter(tr) {
    if (!state.filterQuery) return true;
    const q = state.filterQuery.toLowerCase();
    return (tr.title && tr.title.toLowerCase().includes(q)) ||
      (tr.artist && tr.artist.toLowerCase().includes(q));
  }

  function renderActiveDrawerView(scrollActiveIntoView = false) {
    updateTabsUI();
    playlistList.innerHTML = '';

    if (state.activeTab === 'saved') {
      renderSavedPlaylists();
      return;
    }

    const customName = getSelectedSavedPlaylistName();
    if (customName) {
      const sp = findSavedPlaylist(customName);
      if (sp) {
        const tracks = Array.isArray(sp.tracks) ? sp.tracks : [];
        const filtered = tracks.filter(matchesFilter);
        playlistEmpty.textContent = tracks.length === 0
          ? `"${sp.name}" is empty. Click + on any song in Queue to add tracks here.`
          : 'No songs match your filter.';
        playlistEmpty.classList.toggle('hidden', filtered.length > 0);
        const frag = document.createDocumentFragment();
        filtered.forEach((tr, idx) => frag.appendChild(createCustomPlaylistRow(sp.name, tr, idx)));
        playlistList.appendChild(frag);
        return;
      }
    }

    if (state.activeTab === 'liked') {
      const items = state.likedTracks.filter(matchesFilter);
      playlistEmpty.textContent = state.likedTracks.length === 0
        ? 'No liked songs yet. Click the heart icon on any song to save it.'
        : 'No liked songs match your filter.';
      playlistEmpty.classList.toggle('hidden', items.length > 0);
      const frag = document.createDocumentFragment();
      items.forEach((tr, idx) => frag.appendChild(createLikedRow(tr, idx)));
      playlistList.appendChild(frag);
      return;
    }

    // Queue tab
    const filtered = [];
    state.playlist.forEach((tr, idx) => {
      if (matchesFilter(tr)) filtered.push({ tr, idx });
    });

    playlistEmpty.textContent = state.playlist.length === 0
      ? 'Search for any song or artist above to build a 30-song playlist.'
      : 'No queued songs match your filter.';
    playlistEmpty.classList.toggle('hidden', filtered.length > 0);

    const frag = document.createDocumentFragment();
    filtered.forEach(({ tr, idx }) => frag.appendChild(createQueueRow(tr, idx)));
    playlistList.appendChild(frag);

    if (scrollActiveIntoView && state.currentIndex >= 0 && !state.filterQuery) {
      const activeEl = playlistList.querySelector('.track-item.active');
      if (activeEl) activeEl.scrollIntoView({ block: 'nearest' });
    }
    updateNowPlayingHeader();
  }

  function appendPlaylistItems(newTracks, startIndex) {
    if (state.activeTab !== 'queue' || state.filterQuery) {
      updateTabsUI();
      updateNowPlayingHeader();
      return;
    }
    playlistEmpty.classList.add('hidden');
    const frag = document.createDocumentFragment();
    newTracks.forEach((tr, offset) => {
      frag.appendChild(createQueueRow(tr, startIndex + offset));
    });
    playlistList.appendChild(frag);
    updateTabsUI();
    updateNowPlayingHeader();
  }

  function makeActionButton(className, title, text, onClick) {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = className;
    btn.title = title;
    btn.textContent = text;
    btn.addEventListener('click', (e) => {
      e.stopPropagation();
      onClick(btn, e);
    });
    return btn;
  }

  function makeLikeRowButton(tr) {
    const liked = isTrackLiked(tr.id);
    return makeActionButton(
      'row-act-btn' + (liked ? ' liked' : ''),
      liked ? 'Unlike' : 'Like',
      liked ? '♥' : '♡',
      () => toggleTrackLike(tr)
    );
  }

  function makeAddPlaylistRowButton(tr) {
    return makeActionButton(
      'row-act-btn add-pl-btn',
      'Add to playlist',
      '+',
      (btn) => openAddToPlaylistPopover(tr, btn)
    );
  }

  function makeRadioRowButton(tr) {
    return makeActionButton(
      'row-act-btn',
      'Start 30-song Radio Mix from this song',
      '✦',
      () => startTrackRadio(tr)
    );
  }

  function attachRowDragHandlers(row, idx, onReorder) {
    row.draggable = true;
    row.addEventListener('dragstart', (e) => {
      if (state.filterQuery) {
        e.preventDefault();
        return;
      }
      state.dragSourceIdx = idx;
      row.classList.add('dragging');
      if (e.dataTransfer) {
        e.dataTransfer.effectAllowed = 'move';
        e.dataTransfer.setData('text/plain', String(idx));
      }
    });
    row.addEventListener('dragend', () => {
      state.dragSourceIdx = -1;
      row.classList.remove('dragging', 'drag-over-top', 'drag-over-bottom');
      playlistList.querySelectorAll('.drag-over-top, .drag-over-bottom').forEach((el) => {
        el.classList.remove('drag-over-top', 'drag-over-bottom');
      });
    });
    row.addEventListener('dragover', (e) => {
      if (state.dragSourceIdx < 0 || state.dragSourceIdx === idx || state.filterQuery) return;
      e.preventDefault();
      const rect = row.getBoundingClientRect();
      const after = (e.clientY - rect.top) > rect.height / 2;
      row.classList.toggle('drag-over-top', !after);
      row.classList.toggle('drag-over-bottom', after);
    });
    row.addEventListener('dragleave', () => {
      row.classList.remove('drag-over-top', 'drag-over-bottom');
    });
    row.addEventListener('drop', (e) => {
      e.preventDefault();
      const from = state.dragSourceIdx;
      state.dragSourceIdx = -1;
      row.classList.remove('drag-over-top', 'drag-over-bottom');
      if (from < 0 || from === idx || state.filterQuery) return;
      const rect = row.getBoundingClientRect();
      const after = (e.clientY - rect.top) > rect.height / 2;
      let to = after ? idx + 1 : idx;
      if (from < to) to--;
      if (from !== to) {
        onReorder(from, to);
      }
    });
  }

  function buildTrackRow({ tr, displayIndex, active, buttons, onRowClick, datasetIndex }) {
    const row = document.createElement('div');
    row.className = 'track-item' + (active ? ' active' : '');
    if (datasetIndex !== undefined) {
      row.dataset.index = String(datasetIndex);
    }

    const idxEl = document.createElement('div');
    idxEl.className = 'track-idx';
    idxEl.textContent = active ? '▶' : String(displayIndex + 1);

    const img = document.createElement('img');
    img.className = 'track-item-thumb';
    img.loading = 'lazy';
    img.src = tr.thumbnail || '';
    img.alt = '';

    const info = document.createElement('div');
    info.className = 'track-item-info';
    const titleEl = document.createElement('div');
    titleEl.className = 'track-item-title';
    titleEl.textContent = tr.title;
    const artistEl = document.createElement('div');
    artistEl.className = 'track-item-artist';
    artistEl.textContent = tr.artist;
    info.appendChild(titleEl);
    info.appendChild(artistEl);

    const dur = document.createElement('div');
    dur.className = 'track-item-dur';
    dur.textContent = tr.duration_text || formatTime(tr.duration_sec);

    const actions = document.createElement('div');
    actions.className = 'track-actions';
    buttons.forEach((btn) => actions.appendChild(btn));

    row.appendChild(idxEl);
    row.appendChild(img);
    row.appendChild(info);
    row.appendChild(dur);
    row.appendChild(actions);

    row.addEventListener('click', onRowClick);
    return row;
  }

  function isTrackCurrentlyPlaying(tr) {
    return state.currentIndex >= 0 && state.playlist[state.currentIndex]?.id === tr.id;
  }

  function createQueueRow(tr, idx) {
    const row = buildTrackRow({
      tr,
      displayIndex: idx,
      active: idx === state.currentIndex,
      datasetIndex: idx,
      buttons: [
        makeLikeRowButton(tr),
        makeAddPlaylistRowButton(tr),
        makeRadioRowButton(tr),
        makeActionButton('row-act-btn', 'Play next', '⤵', () => moveTrackPlayNext(idx)),
        makeActionButton('row-act-btn danger', 'Remove from queue', '×', () => removeTrackFromQueue(idx)),
      ],
      onRowClick: () => playTrackAt(idx, true),
    });
    attachRowDragHandlers(row, idx, moveQueueTrackTo);
    return row;
  }

  function createLikedRow(tr, idx) {
    return buildTrackRow({
      tr,
      displayIndex: idx,
      active: isTrackCurrentlyPlaying(tr),
      buttons: [
        makeAddPlaylistRowButton(tr),
        makeRadioRowButton(tr),
        makeActionButton('row-act-btn liked', 'Remove from Liked', '♥', () => toggleTrackLike(tr)),
      ],
      onRowClick: () => playTrackFromCollection(tr),
    });
  }

  function createCustomPlaylistRow(playlistName, tr, idx) {
    const row = buildTrackRow({
      tr,
      displayIndex: idx,
      active: isTrackCurrentlyPlaying(tr),
      buttons: [
        makeLikeRowButton(tr),
        makeAddPlaylistRowButton(tr),
        makeRadioRowButton(tr),
        makeActionButton('row-act-btn danger', `Remove from "${playlistName}"`, '×', () => removeTrackFromSavedPlaylist(playlistName, tr)),
      ],
      onRowClick: () => playTrackFromCollection(tr),
    });
    attachRowDragHandlers(row, idx, (from, to) => moveCustomPlaylistTrack(playlistName, from, to));
    return row;
  }

  async function moveQueueTrackTo(fromIdx, toIdx) {
    if (fromIdx < 0 || fromIdx >= state.playlist.length || toIdx < 0 || toIdx >= state.playlist.length || fromIdx === toIdx) return;
    const [moved] = state.playlist.splice(fromIdx, 1);
    state.playlist.splice(toIdx, 0, moved);
    if (state.currentIndex === fromIdx) {
      state.currentIndex = toIdx;
    } else if (fromIdx < state.currentIndex && toIdx >= state.currentIndex) {
      state.currentIndex--;
    } else if (fromIdx > state.currentIndex && toIdx <= state.currentIndex) {
      state.currentIndex++;
    }
    state.shuffleHistory = [];
    renderActiveDrawerView(false);
    await window.backendUpdatePlaylist(state.playlist, state.currentIndex).catch(() => {});
  }

  async function moveCustomPlaylistTrack(playlistName, fromIdx, toIdx) {
    try {
      const updated = await window.backendMovePlaylistTrack(playlistName, fromIdx, toIdx);
      state.savedPlaylists = Array.isArray(updated) ? updated : [];
      renderActiveDrawerView(false);
    } catch (_) {
      showToast('Failed to reorder playlist');
    }
  }

  async function startTrackRadio(tr) {
    if (!tr || !tr.id || state.isSearching) return;
    state.isSearching = true;
    clearErrorSkipTimer();
    searchBtnIcon.classList.add('hidden');
    searchSpinner.classList.remove('hidden');
    closeAddToPlaylistPopover();
    try {
      const res = await window.backendStartRadio(tr);
      if (!res || !Array.isArray(res.playlist) || res.playlist.length === 0) {
        showToast('Could not build Radio Mix');
        return;
      }
      state.playlist = res.playlist;
      state.shuffleHistory = [];
      state.shuffleBag = [];
      state.consecutiveErrors = 0;
      state.currentIndex = 0;
      state.activeTab = 'queue';
      state.filterQuery = '';
      playlistFilter.value = '';
      searchInput.value = `Mix: ${tr.title || ''}`.trim();
      renderActiveDrawerView(true);
      showToast(res.label || `Radio Mix (${res.playlist.length} songs)`);
      await playTrackAt(0, true, 0);
    } catch (_) {
      showToast('Failed to start Radio Mix');
    } finally {
      state.isSearching = false;
      searchBtnIcon.classList.remove('hidden');
      searchSpinner.classList.add('hidden');
    }
  }

  function renderSavedPlaylists() {
    playlistEmpty.textContent = 'No playlists yet. Click "+ Playlist" to save your queue or create a playlist.';
    playlistEmpty.classList.toggle('hidden', state.savedPlaylists.length > 0);
    if (state.savedPlaylists.length === 0) return;

    const frag = document.createDocumentFragment();
    state.savedPlaylists.forEach((sp) => {
      const card = document.createElement('div');
      card.className = 'saved-playlist-item';

      const meta = document.createElement('div');
      meta.className = 'saved-playlist-meta';
      const name = document.createElement('div');
      name.className = 'saved-playlist-name';
      name.textContent = sp.name;
      const sub = document.createElement('div');
      sub.className = 'saved-playlist-sub';
      const count = Array.isArray(sp.tracks) ? sp.tracks.length : 0;
      sub.textContent = `${count} track${count === 1 ? '' : 's'}${sp.query ? ' • ' + sp.query : ''}`;
      meta.appendChild(name);
      meta.appendChild(sub);

      const acts = document.createElement('div');
      acts.className = 'saved-playlist-actions';

      const appendBtn = makeActionButton('row-act-btn', 'Append to queue', '+Q', async () => {
        await appendTracksToQueue(sp.tracks || [], sp.name);
      });

      const renameBtn = makeActionButton('row-act-btn', 'Rename playlist', '✎', () => {
        const inp = document.createElement('input');
        inp.type = 'text';
        inp.className = 'saved-rename-input';
        inp.value = sp.name;
        let finished = false;
        const finishRename = async (commit) => {
          if (finished) return;
          finished = true;
          const nextName = inp.value.trim();
          if (commit && nextName && nextName !== sp.name) {
            try {
              const updated = await window.backendRenamePlaylist(sp.name, nextName);
              state.savedPlaylists = Array.isArray(updated) ? updated : [];
              showToast(`Renamed to "${nextName}"`);
            } catch (_) {
              showToast('Name already exists or invalid');
            }
          }
          renderActiveDrawerView(false);
        };
        inp.addEventListener('click', (ev) => ev.stopPropagation());
        inp.addEventListener('keydown', (ev) => {
          ev.stopPropagation();
          if (ev.key === 'Enter') {
            ev.preventDefault();
            finishRename(true);
          } else if (ev.key === 'Escape') {
            ev.preventDefault();
            finishRename(false);
          }
        });
        inp.addEventListener('blur', () => finishRename(true));
        name.replaceWith(inp);
        inp.focus();
        inp.select();
      });

      const delBtn = document.createElement('button');
      delBtn.type = 'button';
      delBtn.className = 'row-act-btn danger';
      delBtn.title = 'Delete saved playlist';
      delBtn.textContent = '×';
      delBtn.addEventListener('click', async (e) => {
        e.stopPropagation();
        try {
          const updated = await window.backendDeleteSavedPlaylist(sp.name);
          state.savedPlaylists = Array.isArray(updated) ? updated : [];
          renderActiveDrawerView();
          showToast(`Deleted "${sp.name}"`);
        } catch (_) {}
      });

      acts.appendChild(appendBtn);
      acts.appendChild(renameBtn);
      acts.appendChild(delBtn);

      card.appendChild(meta);
      card.appendChild(acts);

      card.addEventListener('click', async () => {
        try {
          const res = await window.backendLoadSavedPlaylist(sp.name);
          if (!res || !Array.isArray(res.playlist) || res.playlist.length === 0) return;
          state.playlist = res.playlist;
          state.shuffleHistory = [];
      state.shuffleBag = [];
          state.currentIndex = 0;
          state.activeTab = 'queue';
          searchInput.value = sp.query || '';
          renderActiveDrawerView(true);
          showToast(`Loaded "${sp.name}"`);
          await playTrackAt(0, true, 0);
        } catch (_) {
          showToast('Failed to load playlist');
        }
      });

      frag.appendChild(card);
    });
    playlistList.appendChild(frag);
  }

  function updateActiveRowHighlight() {
    if (state.activeTab !== 'queue') {
      if (state.activeTab === 'liked' || state.activeTab.startsWith(CUSTOM_TAB_PREFIX)) {
        renderActiveDrawerView(false);
      }
      return;
    }
    const children = playlistList.children;
    for (let i = 0; i < children.length; i++) {
      const el = children[i];
      const idx = parseInt(el.dataset.index || '-1', 10);
      const active = idx === state.currentIndex;
      el.classList.toggle('active', active);
      const idxEl = el.querySelector('.track-idx');
      if (idxEl) idxEl.textContent = active ? '▶' : String(idx + 1);
      if (active && !state.filterQuery) {
        el.scrollIntoView({ block: 'nearest' });
      }
    }
  }

  function positionFloatingPopover(popEl, anchorEl) {
    if (!popEl) return;
    popEl.style.visibility = 'hidden';
    popEl.classList.remove('hidden');
    const popRect = popEl.getBoundingClientRect();
    const anchorRect = anchorEl ? anchorEl.getBoundingClientRect() : null;
    let top = anchorRect ? (anchorRect.bottom + 6) : Math.max(10, (window.innerHeight - popRect.height) / 2);
    if (anchorRect && top + popRect.height > window.innerHeight - 8) {
      const above = anchorRect.top - popRect.height - 6;
      if (above >= 8) {
        top = above;
      }
    }
    top = Math.max(8, Math.min(Math.max(8, window.innerHeight - popRect.height - 8), top));
    popEl.style.top = `${Math.round(top)}px`;
    popEl.style.visibility = 'visible';
  }

  function pickNextShuffleIndex() {
    const n = state.playlist.length;
    if (n <= 1) return 0;
    // Prune out-of-range or currently playing indices from existing bag.
    state.shuffleBag = state.shuffleBag.filter((i) => i >= 0 && i < n && i !== state.currentIndex);
    if (state.shuffleBag.length === 0) {
      for (let i = 0; i < n; i++) {
        if (i !== state.currentIndex) state.shuffleBag.push(i);
      }
      for (let i = state.shuffleBag.length - 1; i > 0; i--) {
        const j = Math.floor(Math.random() * (i + 1));
        [state.shuffleBag[i], state.shuffleBag[j]] = [state.shuffleBag[j], state.shuffleBag[i]];
      }
    }
    return state.shuffleBag.pop();
  }

  function clearErrorSkipTimer() {
    if (state.errorSkipTimer) {
      clearTimeout(state.errorSkipTimer);
      state.errorSkipTimer = null;
    }
  }

  async function toggleTrackLike(tr) {
    if (!tr || !tr.id) return;
    try {
      const updated = await window.backendToggleLike(tr);
      state.likedTracks = Array.isArray(updated) ? updated : [];
      updateLikeButtonUI();
      renderActiveDrawerView(false);
      if (state.popoverTrack && state.popoverTrack.id === tr.id && !addPlaylistPopover.classList.contains('hidden')) {
        renderAddToPlaylistList(state.popoverTrack);
      }
    } catch (_) {}
  }

  async function removeTrackFromSavedPlaylist(playlistName, tr) {
    if (!playlistName || !tr || !tr.id) return;
    try {
      const updated = await window.backendRemoveTrackFromPlaylist(playlistName, tr.id);
      state.savedPlaylists = Array.isArray(updated) ? updated : [];
      renderActiveDrawerView(false);
      showToast(`Removed from "${playlistName}"`);
    } catch (_) {
      showToast('Failed to remove track');
    }
  }

  function closeAddToPlaylistPopover() {
    state.popoverTrack = null;
    if (addPlaylistPopover) {
      addPlaylistPopover.classList.add('hidden');
    }
  }

  function renderAddToPlaylistList(tr) {
    if (!addPlaylistList) return;
    addPlaylistList.innerHTML = '';
    const frag = document.createDocumentFragment();

    const makeOption = (label, countText, inPlaylist, onToggle) => {
      const item = document.createElement('button');
      item.type = 'button';
      item.className = 'add-playlist-item' + (inPlaylist ? ' in-playlist' : '');
      const check = document.createElement('span');
      check.className = 'add-playlist-check';
      check.textContent = inPlaylist ? '✓' : '';
      const nameEl = document.createElement('span');
      nameEl.className = 'add-playlist-item-name';
      nameEl.textContent = label;
      const countEl = document.createElement('span');
      countEl.className = 'add-playlist-item-count';
      countEl.textContent = countText;
      item.appendChild(check);
      item.appendChild(nameEl);
      item.appendChild(countEl);
      item.addEventListener('click', (e) => {
        e.stopPropagation();
        onToggle();
      });
      return item;
    };

    frag.appendChild(makeOption(
      'Liked ♥',
      String(state.likedTracks.length),
      isTrackLiked(tr.id),
      () => toggleTrackLike(tr)
    ));

    state.savedPlaylists.forEach((sp) => {
      const count = Array.isArray(sp.tracks) ? sp.tracks.length : 0;
      const inPl = playlistContainsTrack(sp, tr.id);
      frag.appendChild(makeOption(
        sp.name,
        String(count),
        inPl,
        async () => {
          try {
            const updated = inPl
              ? await window.backendRemoveTrackFromPlaylist(sp.name, tr.id)
              : await window.backendAddTrackToPlaylist(sp.name, tr);
            state.savedPlaylists = Array.isArray(updated) ? updated : [];
            renderAddToPlaylistList(tr);
            renderActiveDrawerView(false);
            showToast(inPl ? `Removed from "${sp.name}"` : `Added to "${sp.name}"`);
          } catch (_) {
            showToast('Failed to update playlist');
          }
        }
      ));
    });

    addPlaylistList.appendChild(frag);
  }

  function openAddToPlaylistPopover(tr, anchorEl) {
    if (!tr || !tr.id || !addPlaylistPopover) return;
    const sameTrackOpen = !addPlaylistPopover.classList.contains('hidden') &&
      state.popoverTrack && state.popoverTrack.id === tr.id;
    if (sameTrackOpen) {
      closeAddToPlaylistPopover();
      return;
    }

    state.popoverTrack = tr;
    addPlaylistTrack.textContent = tr.title || 'Selected track';
    addPlaylistInput.value = '';
    renderAddToPlaylistList(tr);

    addPlaylistPopover.style.visibility = 'hidden';
    addPlaylistPopover.classList.remove('hidden');

    const popRect = addPlaylistPopover.getBoundingClientRect();
    const btnRect = anchorEl.getBoundingClientRect();
    let top = btnRect.bottom + 6;
    if (top + popRect.height > window.innerHeight - 10) {
      top = btnRect.top - popRect.height - 6;
    }
    top = Math.max(10, Math.min(window.innerHeight - popRect.height - 10, top));
    addPlaylistPopover.style.top = `${Math.round(top)}px`;
    addPlaylistPopover.style.visibility = 'visible';
  }

  async function moveTrackPlayNext(idx) {
    if (idx < 0 || idx >= state.playlist.length || idx === state.currentIndex) return;
    const [tr] = state.playlist.splice(idx, 1);
    if (idx < state.currentIndex) {
      state.currentIndex--;
    }
    const insertAt = Math.min(state.playlist.length, Math.max(0, state.currentIndex + 1));
    state.playlist.splice(insertAt, 0, tr);
    state.shuffleHistory = [];
    renderActiveDrawerView(false);
    showToast(`Queued next: ${tr.title}`);
    await window.backendUpdatePlaylist(state.playlist, state.currentIndex).catch(() => {});
  }

  async function removeTrackFromQueue(idx) {
    if (idx < 0 || idx >= state.playlist.length) return;
    const removedActive = idx === state.currentIndex;
    state.playlist.splice(idx, 1);
    state.shuffleHistory = [];

    if (state.playlist.length === 0) {
      state.currentIndex = -1;
      state.userInitiatedPlay = false;
      clearErrorSkipTimer();
      videoLoading.classList.add('hidden');
      media.pause();
      media.removeAttribute('src');
      media.load();
      updatePlayButtonUI();
      renderActiveDrawerView(false);
      await window.backendUpdatePlaylist([], -1).catch(() => {});
      return;
    }

    if (idx < state.currentIndex) {
      state.currentIndex--;
    } else if (removedActive) {
      const nextIdx = Math.min(idx, state.playlist.length - 1);
      state.currentIndex = nextIdx;
      renderActiveDrawerView(false);
      await window.backendUpdatePlaylist(state.playlist, nextIdx).catch(() => {});
      await playTrackAt(nextIdx, true);
      return;
    }

    renderActiveDrawerView(false);
    await window.backendUpdatePlaylist(state.playlist, state.currentIndex).catch(() => {});
  }

  async function appendTracksToQueue(tracks, sourceLabel) {
    if (!Array.isArray(tracks) || tracks.length === 0) return;
    const existing = new Set(state.playlist.map((t) => t.id));
    const uniqueNew = tracks.filter((t) => t && t.id && !existing.has(t.id));
    if (uniqueNew.length === 0) {
      showToast('All songs are already in Queue');
      return;
    }
    const wasEmpty = state.playlist.length === 0;
    state.playlist.push(...uniqueNew.map((t) => ({ ...t })));
    if (wasEmpty) {
      state.currentIndex = 0;
    }
    renderActiveDrawerView(false);
    await window.backendUpdatePlaylist(state.playlist, state.currentIndex).catch(() => {});
    showToast(`Added ${uniqueNew.length} song${uniqueNew.length === 1 ? '' : 's'} to Queue` + (sourceLabel ? ` from "${sourceLabel}"` : ''));
    if (wasEmpty) {
      await playTrackAt(0, true, 0);
    }
  }

  async function playTrackFromCollection(tr) {
    let idx = state.playlist.findIndex((item) => item.id === tr.id);
    if (idx < 0) {
      const insertAt = state.currentIndex >= 0 ? state.currentIndex + 1 : 0;
      state.playlist.splice(insertAt, 0, tr);
      idx = insertAt;
      await window.backendUpdatePlaylist(state.playlist, idx).catch(() => {});
    }
    await playTrackAt(idx, true);
    renderActiveDrawerView(false);
  }

  async function playTrackAt(index, autoplay = true, startPosSec = 0) {
    if (index < 0 || index >= state.playlist.length) return;
    clearErrorSkipTimer();
    const seq = ++state.playbackSeq;
    state.currentIndex = index;
    state.pendingSeekSec = startPosSec;
    state.userInitiatedPlay = autoplay;
    updateActiveRowHighlight();
    updateNowPlayingHeader();

    videoLoading.classList.remove('hidden');
    try {
      const streamURL = await window.backendSelectTrack(index);
      if (seq !== state.playbackSeq) return;
      if (!streamURL) {
        videoLoading.classList.add('hidden');
        return;
      }
      media.src = streamURL;
      media.playbackRate = state.playbackRate;
      media.load();
      if (autoplay) {
        media.play().catch(() => {
          updatePlayButtonUI();
        });
      } else {
        updatePlayButtonUI();
      }
      pushPlaybackState();
    } catch (err) {
      if (seq !== state.playbackSeq) return;
      videoLoading.classList.add('hidden');
      showToast('Failed to load track');
    }
  }

  function togglePlayPause() {
    if (state.currentIndex < 0 && state.playlist.length > 0) {
      playTrackAt(0, true);
      return;
    }
    if (!media.src) return;
    if (media.paused) {
      state.userInitiatedPlay = true;
      media.play().catch(() => {});
    } else {
      state.userInitiatedPlay = false;
      media.pause();
    }
  }

  async function playNext(fromEnded = false) {
    if (state.playlist.length === 0) return;

    if (fromEnded && state.repeatMode === 'one') {
      media.currentTime = 0;
      media.play().catch(() => {});
      return;
    }

    if (state.shuffle) {
      if (state.currentIndex >= 0) {
        state.shuffleHistory.push(state.currentIndex);
        if (state.shuffleHistory.length > 100) state.shuffleHistory.shift();
      }
      const nextIdx = pickNextShuffleIndex();
      await playTrackAt(nextIdx, true);
      return;
    }

    const next = state.currentIndex + 1;
    if (next < state.playlist.length) {
      await playTrackAt(next, true);
      if (next >= state.playlist.length - 3) {
        triggerLoadMore();
      }
      return;
    }

    // Reached end of queue: try loading more songs automatically first.
    const addedCount = await triggerLoadMore();
    if (addedCount > 0 && state.currentIndex + 1 < state.playlist.length) {
      await playTrackAt(state.currentIndex + 1, true);
      return;
    }

    if (state.repeatMode === 'all' && state.playlist.length > 0) {
      await playTrackAt(0, true);
    }
  }

  function playPrev() {
    if (state.playlist.length === 0) return;
    if (media.currentTime > 3) {
      media.currentTime = 0;
      pushPlaybackState();
      return;
    }
    if (state.shuffle && state.shuffleHistory.length > 0) {
      const prevIdx = state.shuffleHistory.pop();
      if (prevIdx >= 0 && prevIdx < state.playlist.length) {
        playTrackAt(prevIdx, true);
        return;
      }
    }
    const prev = state.currentIndex - 1;
    if (prev >= 0) {
      playTrackAt(prev, true);
    } else if (state.repeatMode === 'all') {
      playTrackAt(state.playlist.length - 1, true);
    }
  }

  async function triggerLoadMore() {
    if (state.isLoadingMore || state.playlist.length === 0) return 0;
    state.isLoadingMore = true;
    loadMoreFooter.classList.remove('hidden');
    try {
      const res = await window.backendLoadMore();
      if (res && Array.isArray(res.added) && res.added.length > 0) {
        const startIdx = state.playlist.length;
        state.playlist.push(...res.added);
        appendPlaylistItems(res.added, startIdx);
        showToast(`Added ${res.added.length} more songs`);
        return res.added.length;
      }
    } catch (_) {
      // ignore transient continuation error
    } finally {
      state.isLoadingMore = false;
      loadMoreFooter.classList.add('hidden');
    }
    return 0;
  }

  function pushPlaybackState() {
    if (typeof window.backendUpdatePlaybackState !== 'function') return Promise.resolve();
    flushSessionPlayTime();
    const pos = Number.isFinite(media.currentTime) ? media.currentTime : 0;
    const playing = !media.paused && !media.ended;
    state.lastSyncMs = Date.now();
    return window.backendUpdatePlaybackState(
      state.currentIndex,
      pos,
      playing,
      state.volume,
      state.muted,
      state.sessionPlayMs / 1000
    ).catch(() => {});
  }

  async function toggleShuffle() {
    state.shuffle = !state.shuffle;
    updateModesUI();
    showToast(state.shuffle ? 'Shuffle: On' : 'Shuffle: Off');
    await window.backendSetModes(state.shuffle, state.repeatMode).catch(() => {});
  }

  async function cycleRepeat() {
    state.repeatMode = state.repeatMode === 'off' ? 'all' : state.repeatMode === 'all' ? 'one' : 'off';
    updateModesUI();
    const label = state.repeatMode === 'one' ? 'Repeat: One' : state.repeatMode === 'all' ? 'Repeat: All' : 'Repeat: Off';
    showToast(label);
    await window.backendSetModes(state.shuffle, state.repeatMode).catch(() => {});
  }

  async function toggleTheme() {
    const next = state.theme === 'light' ? 'dark' : 'light';
    applyThemeUI(next);
    showToast(next === 'light' ? 'YouTube Light theme' : 'YouTube Dark theme');
    await window.backendSetTheme(next).catch(() => {});
  }

  function setupMediaSession() {
    if (!('mediaSession' in navigator)) return;
    const ms = navigator.mediaSession;
    try {
      ms.setActionHandler('play', () => {
        if (media.paused) togglePlayPause();
      });
      ms.setActionHandler('pause', () => {
        if (!media.paused) togglePlayPause();
      });
      ms.setActionHandler('nexttrack', () => playNext(false));
      ms.setActionHandler('previoustrack', () => playPrev());
      ms.setActionHandler('seekto', (details) => {
        if (details && Number.isFinite(details.seekTime) && Number.isFinite(media.duration)) {
          media.currentTime = Math.max(0, Math.min(media.duration, details.seekTime));
          pushPlaybackState();
        }
      });
    } catch (_) {}
  }

  // --- Search Dropdown (Recent Searches + Live Autocomplete) ---
  function closeSearchDropdown() {
    if (searchDropdown) {
      searchDropdown.classList.add('hidden');
    }
  }

  function renderSearchDropdown(suggestions = []) {
    if (!searchDropdown) return;
    const typed = searchInput.value.trim().toLowerCase();
    const matchingRecent = state.recentSearches.filter((q) => !typed || q.toLowerCase().includes(typed));
    const filteredSuggest = suggestions.filter(
      (s) => !matchingRecent.some((r) => r.toLowerCase() === s.toLowerCase())
    );

    if (matchingRecent.length === 0 && filteredSuggest.length === 0) {
      closeSearchDropdown();
      return;
    }

    searchDropdown.innerHTML = '';
    const frag = document.createDocumentFragment();

    const makeSuggestRow = (text, isRecent) => {
      const row = document.createElement('div');
      row.className = 'search-suggest-row';
      const badge = document.createElement('span');
      badge.className = 'search-suggest-badge';
      badge.textContent = isRecent ? '🕒' : '🔍';
      const label = document.createElement('span');
      label.className = 'search-suggest-text';
      label.textContent = text;
      row.appendChild(badge);
      row.appendChild(label);

      if (isRecent) {
        const del = document.createElement('button');
        del.type = 'button';
        del.className = 'search-suggest-del';
        del.title = 'Remove from recent searches';
        del.textContent = '×';
        del.addEventListener('click', async (ev) => {
          ev.stopPropagation();
          try {
            const next = await window.backendRemoveRecentSearch(text);
            state.recentSearches = Array.isArray(next) ? next : [];
            renderSearchDropdown(suggestions);
          } catch (_) {}
        });
        row.appendChild(del);
      }

      row.addEventListener('click', () => {
        searchInput.value = text;
        closeSearchDropdown();
        runSearchQuery(text);
      });
      return row;
    };

    matchingRecent.slice(0, 5).forEach((q) => frag.appendChild(makeSuggestRow(q, true)));
    filteredSuggest.slice(0, 6).forEach((s) => frag.appendChild(makeSuggestRow(s, false)));
    searchDropdown.appendChild(frag);
    searchDropdown.classList.remove('hidden');
  }

  async function runSearchQuery(q) {
    if (!q || state.isSearching) return;
    closeSearchDropdown();
    state.isSearching = true;
    clearErrorSkipTimer();
    searchBtnIcon.classList.add('hidden');
    searchSpinner.classList.remove('hidden');
    try {
      const res = await window.backendSearch(q);
      if (!res || !Array.isArray(res.playlist) || res.playlist.length === 0) {
        showToast('No playable songs found');
        return;
      }
      state.playlist = res.playlist;
      if (Array.isArray(res.recent_searches)) {
        state.recentSearches = res.recent_searches;
      }
      state.shuffleHistory = [];
      state.shuffleBag = [];
      state.consecutiveErrors = 0;
      state.currentIndex = 0;
      state.activeTab = 'queue';
      state.filterQuery = '';
      playlistFilter.value = '';
      renderActiveDrawerView(true);
      showToast(res.label || `Built ${state.playlist.length}-song playlist`);
      await playTrackAt(0, true, 0);
    } catch (err) {
      showToast('Search failed: ' + (err || 'network error'));
    } finally {
      state.isSearching = false;
      searchBtnIcon.classList.remove('hidden');
      searchSpinner.classList.add('hidden');
    }
  }

  searchForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    await runSearchQuery(searchInput.value.trim());
  });

  searchInput.addEventListener('focus', () => {
    renderSearchDropdown([]);
  });

  searchInput.addEventListener('input', () => {
    const q = searchInput.value.trim();
    if (state.searchSuggestTimer) {
      clearTimeout(state.searchSuggestTimer);
      state.searchSuggestTimer = null;
    }
    if (!q) {
      renderSearchDropdown([]);
      return;
    }
    renderSearchDropdown([]);
    state.searchSuggestTimer = setTimeout(async () => {
      try {
        const list = await window.backendSearchSuggest(q);
        if (document.activeElement === searchInput && searchInput.value.trim() === q) {
          renderSearchDropdown(Array.isArray(list) ? list : []);
        }
      } catch (_) {}
    }, 200);
  });

  if (trackCounter) {
    trackCounter.addEventListener('click', () => {
      if (state.currentIndex < 0 || state.playlist.length === 0) return;
      state.activeTab = 'queue';
      state.filterQuery = '';
      playlistFilter.value = '';
      renderActiveDrawerView(true);
    });
  }

  likeBtn.addEventListener('click', async () => {
    if (state.currentIndex < 0 || state.currentIndex >= state.playlist.length) return;
    await toggleTrackLike(state.playlist[state.currentIndex]);
  });

  shuffleBtn.addEventListener('click', toggleShuffle);
  repeatBtn.addEventListener('click', cycleRepeat);
  playBtn.addEventListener('click', togglePlayPause);
  nextBtn.addEventListener('click', () => playNext(false));
  prevBtn.addEventListener('click', playPrev);

  muteBtn.addEventListener('click', () => {
    state.muted = !state.muted;
    updateVolumeUI();
    pushPlaybackState();
  });

  volumeRange.addEventListener('input', () => {
    state.volume = parseFloat(volumeRange.value);
    if (state.volume > 0 && state.muted) {
      state.muted = false;
    }
    updateVolumeUI();
  });

  volumeRange.addEventListener('change', pushPlaybackState);

  videoToggleBtn.addEventListener('click', async () => {
    const next = !state.videoEnabled;
    applyVideoModeUI(next);
    await window.backendToggleVideo(next);
  });

  playlistToggleBtn.addEventListener('click', async () => {
    const next = !state.playlistExpanded;
    applyPlaylistModeUI(next);
    await window.backendTogglePlaylist(next);
  });

  pinBtn.addEventListener('click', async () => {
    const next = !state.alwaysOnTop;
    applyAlwaysOnTopUI(next);
    showToast(next ? 'Stay on top: On' : 'Stay on top: Off');
    await window.backendToggleAlwaysOnTop(next);
  });

  themeBtn.addEventListener('click', toggleTheme);

  // --- Audio Popover (EQ, Speed, Sleep Timer, VU Persistence & Shortcuts) ---
  function updateAudioSettingsUI() {
    applyEQPresetGains();
    media.playbackRate = state.playbackRate;

    document.querySelectorAll('.eq-opt').forEach((btn) => {
      btn.classList.toggle('active', btn.dataset.eq === state.eqPreset);
    });
    document.querySelectorAll('.speed-opt').forEach((btn) => {
      btn.classList.toggle('active', parseFloat(btn.dataset.rate) === state.playbackRate);
    });

    if (eqBtnLabel && eqBtn) {
      const sleepRemMs = state.sleepEndMs > Date.now() ? state.sleepEndMs - Date.now() : 0;
      if (sleepRemMs > 0) {
        const mins = Math.max(1, Math.ceil(sleepRemMs / 60000));
        eqBtnLabel.textContent = `🌙${mins}m`;
      } else if (state.playbackRate !== 1) {
        eqBtnLabel.textContent = `${state.playbackRate}×`;
      } else if (state.eqPreset !== 'flat') {
        eqBtnLabel.textContent = state.eqPreset.charAt(0).toUpperCase() + state.eqPreset.slice(1);
      } else {
        eqBtnLabel.textContent = 'EQ';
      }
      eqBtn.classList.toggle('active', sleepRemMs > 0 || state.eqPreset !== 'flat' || state.playbackRate !== 1);
    }
  }

  function updateSleepCountdownUI() {
    const remMs = state.sleepEndMs > Date.now() ? state.sleepEndMs - Date.now() : 0;
    if (sleepCountdownLabel) {
      if (remMs <= 0) {
        sleepCountdownLabel.textContent = 'Off';
      } else {
        const totalSec = Math.ceil(remMs / 1000);
        const m = Math.floor(totalSec / 60);
        const s = totalSec % 60;
        sleepCountdownLabel.textContent = `${m}:${s < 10 ? '0' : ''}${s}`;
      }
    }
    const activeMins = state.sleepEndMs > Date.now() ? Math.ceil((state.sleepEndMs - Date.now()) / 60000) : 0;
    document.querySelectorAll('.sleep-opt').forEach((btn) => {
      const m = parseInt(btn.dataset.mins || '0', 10);
      btn.classList.toggle('active', m === 0 ? activeMins === 0 : (activeMins > 0 && Math.abs(activeMins - m) <= 15 && btn === Array.from(document.querySelectorAll('.sleep-opt')).find((b) => parseInt(b.dataset.mins || '0', 10) >= activeMins)));
    });
    updateAudioSettingsUI();
  }

  window.__onSleepTimerChanged = (remainingSec) => {
    if (state.sleepTickTimer) {
      clearInterval(state.sleepTickTimer);
      state.sleepTickTimer = null;
    }
    if (remainingSec > 0) {
      state.sleepEndMs = Date.now() + remainingSec * 1000;
      state.sleepTickTimer = setInterval(() => {
        if (Date.now() >= state.sleepEndMs) {
          state.sleepEndMs = 0;
          clearInterval(state.sleepTickTimer);
          state.sleepTickTimer = null;
        }
        updateSleepCountdownUI();
      }, 1000);
    } else {
      state.sleepEndMs = 0;
    }
    updateSleepCountdownUI();
  };

  function persistAudioPrefs() {
    if (typeof window.backendSetAudioPrefs === 'function') {
      window.backendSetAudioPrefs(state.vuStyle, state.eqPreset, state.playbackRate).catch(() => {});
    }
  }

  if (vuMeter) {
    vuMeter.addEventListener('click', () => {
      state.vuStyle = state.vuStyle === 'led' ? 'bar' : 'led';
      vuMeter.title = `Stereo VU Meter (${state.vuStyle === 'led' ? 'LED' : 'Bar'}) — Click to switch style`;
      drawVuMeter();
      persistAudioPrefs();
    });
  }

  if (eqBtn && audioPopover) {
    eqBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      const opening = audioPopover.classList.contains('hidden');
      if (opening) {
        closeAddToPlaylistPopover();
        closeSearchDropdown();
        if (authPopover) authPopover.classList.add('hidden');
        if (shortcutsPopover) shortcutsPopover.classList.add('hidden');
        positionFloatingPopover(audioPopover, eqBtn);
        updateSleepCountdownUI();
      } else {
        audioPopover.classList.add('hidden');
      }
    });
  }

  if (audioPopClose && audioPopover) {
    audioPopClose.addEventListener('click', () => audioPopover.classList.add('hidden'));
  }

  document.querySelectorAll('.eq-opt').forEach((btn) => {
    btn.addEventListener('click', () => {
      state.eqPreset = btn.dataset.eq || 'flat';
      updateAudioSettingsUI();
      persistAudioPrefs();
      showToast(`EQ: ${btn.textContent}`);
    });
  });

  document.querySelectorAll('.speed-opt').forEach((btn) => {
    btn.addEventListener('click', () => {
      const rate = parseFloat(btn.dataset.rate || '1');
      if (rate >= 0.5 && rate <= 2.0) {
        state.playbackRate = rate;
        updateAudioSettingsUI();
        persistAudioPrefs();
        showToast(`Speed: ${rate}×`);
      }
    });
  });

  document.querySelectorAll('.sleep-opt').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const mins = parseInt(btn.dataset.mins || '0', 10);
      if (typeof window.backendSetSleepTimer === 'function') {
        await window.backendSetSleepTimer(mins).catch(() => {});
      }
    });
  });

  if (curCopyLinkBtn) {
    curCopyLinkBtn.addEventListener('click', () => {
      const tr = state.currentIndex >= 0 ? state.playlist[state.currentIndex] : null;
      if (tr) copyTrackURL(tr);
      else showToast('No active song');
    });
  }

  if (curOpenYtBtn) {
    curOpenYtBtn.addEventListener('click', () => {
      const tr = state.currentIndex >= 0 ? state.playlist[state.currentIndex] : null;
      if (tr) openTrackExternal(tr);
      else showToast('No active song');
    });
  }

  if (curRadioBtn) {
    curRadioBtn.addEventListener('click', async () => {
      const tr = state.currentIndex >= 0 ? state.playlist[state.currentIndex] : null;
      if (audioPopover) audioPopover.classList.add('hidden');
      if (tr) await startTrackRadio(tr);
      else showToast('No active song');
    });
  }

  if (shortcutsHelpBtn && shortcutsPopover) {
    shortcutsHelpBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      if (audioPopover) audioPopover.classList.add('hidden');
      if (shortcutsPopover.classList.contains('hidden')) {
        positionFloatingPopover(shortcutsPopover, eqBtn);
      } else {
        shortcutsPopover.classList.add('hidden');
      }
    });
  }

  if (shortcutsCloseBtn && shortcutsPopover) {
    shortcutsCloseBtn.addEventListener('click', () => shortcutsPopover.classList.add('hidden'));
  }

  document.addEventListener('pointerdown', unlockAudioContextOnGesture);
  document.addEventListener('keydown', unlockAudioContextOnGesture);

  if (drawerTabsScroll) {
    drawerTabsScroll.addEventListener('wheel', (e) => {
      if (Math.abs(e.deltaY) > Math.abs(e.deltaX)) {
        e.preventDefault();
        drawerTabsScroll.scrollLeft += e.deltaY;
      }
    }, { passive: false });
  }

  tabQueue.addEventListener('click', () => {
    closeAddToPlaylistPopover();
    state.activeTab = 'queue';
    renderActiveDrawerView(true);
  });
  tabLiked.addEventListener('click', () => {
    closeAddToPlaylistPopover();
    state.activeTab = 'liked';
    renderActiveDrawerView(false);
  });
  tabSaved.addEventListener('click', () => {
    closeAddToPlaylistPopover();
    state.activeTab = 'saved';
    renderActiveDrawerView(false);
  });

  playlistFilter.addEventListener('input', () => {
    state.filterQuery = playlistFilter.value.trim();
    renderActiveDrawerView(false);
  });

  playLikedBtn.addEventListener('click', async () => {
    closeAddToPlaylistPopover();
    const customName = getSelectedSavedPlaylistName();
    if (customName) {
      const sp = findSavedPlaylist(customName);
      if (!sp || !Array.isArray(sp.tracks) || sp.tracks.length === 0) return;
      try {
        const res = await window.backendLoadSavedPlaylist(sp.name);
        if (!res || !Array.isArray(res.playlist) || res.playlist.length === 0) return;
        state.playlist = res.playlist;
        state.shuffleHistory = [];
      state.shuffleBag = [];
        state.currentIndex = 0;
        state.activeTab = 'queue';
        searchInput.value = sp.query || '';
        renderActiveDrawerView(true);
        showToast(`Playing "${sp.name}"`);
        await playTrackAt(0, true, 0);
      } catch (_) {
        showToast('Failed to play playlist');
      }
      return;
    }

    if (state.likedTracks.length === 0) return;
    try {
      const res = await window.backendLoadLikedPlaylist();
      if (!res || !Array.isArray(res.playlist) || res.playlist.length === 0) return;
      state.playlist = res.playlist;
      state.shuffleHistory = [];
      state.shuffleBag = [];
      state.currentIndex = 0;
      state.activeTab = 'queue';
      searchInput.value = '';
      renderActiveDrawerView(true);
      showToast('Playing Liked songs');
      await playTrackAt(0, true, 0);
    } catch (_) {
      showToast('Failed to play Liked songs');
    }
  });

  if (clearQueueBtn) {
    clearQueueBtn.addEventListener('click', async () => {
      if (state.playlist.length === 0) return;
      closeAddToPlaylistPopover();
      state.playlist = [];
      state.shuffleHistory = [];
      state.shuffleBag = [];
      state.shuffleBag = [];
      state.currentIndex = -1;
      state.userInitiatedPlay = false;
      clearErrorSkipTimer();
      stopPlaytimeTracker();
      videoLoading.classList.add('hidden');
      media.pause();
      media.removeAttribute('src');
      media.load();
      updatePlayButtonUI();
      renderActiveDrawerView(false);
      await window.backendUpdatePlaylist([], -1).catch(() => {});
      showToast('Queue cleared');
    });
  }

  if (appendCollectionBtn) {
    appendCollectionBtn.addEventListener('click', async () => {
      closeAddToPlaylistPopover();
      const customName = getSelectedSavedPlaylistName();
      if (customName) {
        const sp = findSavedPlaylist(customName);
        if (sp) await appendTracksToQueue(sp.tracks || [], sp.name);
        return;
      }
      if (state.activeTab === 'liked') {
        await appendTracksToQueue(state.likedTracks, 'Liked');
      }
    });
  }

  saveQueueBtn.addEventListener('click', () => {
    saveQueueBar.classList.toggle('hidden');
    if (!saveQueueBar.classList.contains('hidden')) {
      const fallback = `Playlist ${state.savedPlaylists.length + 1}`;
      saveQueueInput.value = state.playlist.length > 0 ? (searchInput.value.trim() || fallback) : fallback;
      updateSaveBarButtonsUI();
      saveQueueInput.focus();
      saveQueueInput.select();
    }
  });

  saveQueueInput.addEventListener('input', updateSaveBarButtonsUI);

  saveQueueCancel.addEventListener('click', () => {
    saveQueueBar.classList.add('hidden');
  });

  async function confirmSaveQueue(mode = 'replace') {
    const name = saveQueueInput.value.trim();
    if (!name || state.playlist.length === 0) return;
    try {
      const updated = await window.backendSavePlaylist(name, mode);
      state.savedPlaylists = Array.isArray(updated) ? updated : [];
      saveQueueBar.classList.add('hidden');
      renderActiveDrawerView(false);
      showToast(mode === 'merge' ? `Merged into "${name}"` : `Saved "${name}"`);
    } catch (err) {
      showToast('Failed to save playlist');
    }
  }

  async function confirmCreateEmptyPlaylist() {
    const name = saveQueueInput.value.trim();
    if (!name) {
      saveQueueInput.focus();
      return;
    }
    try {
      const updated = await window.backendCreatePlaylist(name, []);
      state.savedPlaylists = Array.isArray(updated) ? updated : [];
      saveQueueBar.classList.add('hidden');
      state.activeTab = CUSTOM_TAB_PREFIX + name;
      renderActiveDrawerView(false);
      const activePill = customPlaylistTabs && customPlaylistTabs.querySelector('.tab-pill.active');
      if (activePill) {
        activePill.scrollIntoView({ block: 'nearest', inline: 'nearest' });
      }
      showToast(`Created playlist "${name}"`);
    } catch (_) {
      showToast('Failed to create playlist');
    }
  }

  saveQueueConfirm.addEventListener('click', () => confirmSaveQueue('replace'));
  if (saveQueueMerge) {
    saveQueueMerge.addEventListener('click', () => confirmSaveQueue('merge'));
  }
  if (saveEmptyConfirm) {
    saveEmptyConfirm.addEventListener('click', confirmCreateEmptyPlaylist);
  }
  saveQueueInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      if (state.playlist.length > 0) {
        const exists = Boolean(findSavedPlaylist(saveQueueInput.value.trim()));
        confirmSaveQueue(exists ? 'merge' : 'replace');
      } else {
        confirmCreateEmptyPlaylist();
      }
    } else if (e.key === 'Escape') {
      saveQueueBar.classList.add('hidden');
    }
  });

  if (addPlaylistOpenBtn) {
    addPlaylistOpenBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      if (state.popoverTrack) {
        openTrackExternal(state.popoverTrack);
      }
    });
  }

  if (addPlaylistCopyBtn) {
    addPlaylistCopyBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      if (state.popoverTrack) {
        copyTrackURL(state.popoverTrack);
      }
    });
  }

  if (addPlaylistClose) {
    addPlaylistClose.addEventListener('click', closeAddToPlaylistPopover);
  }

  if (addPlaylistForm) {
    addPlaylistForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const tr = state.popoverTrack;
      const name = addPlaylistInput.value.trim();
      if (!tr || !tr.id || !name) return;
      try {
        const updated = await window.backendCreatePlaylist(name, [tr]);
        state.savedPlaylists = Array.isArray(updated) ? updated : [];
        addPlaylistInput.value = '';
        renderActiveDrawerView(false);
        closeAddToPlaylistPopover();
        showToast(`Created "${name}" & added song`);
      } catch (_) {
        showToast('Failed to create playlist');
      }
    });
  }

  document.addEventListener('click', (e) => {
    if (searchDropdown && !searchDropdown.classList.contains('hidden')) {
      if (!searchDropdown.contains(e.target) && !searchForm.contains(e.target)) {
        closeSearchDropdown();
      }
    }
    if (addPlaylistPopover && !addPlaylistPopover.classList.contains('hidden')) {
      if (!addPlaylistPopover.contains(e.target) && !(e.target && typeof e.target.closest === 'function' && e.target.closest('.add-pl-btn'))) {
        closeAddToPlaylistPopover();
      }
    }
    if (authPopover && !authPopover.classList.contains('hidden')) {
      if (!authPopover.contains(e.target) && !authBtn.contains(e.target)) {
        authPopover.classList.add('hidden');
      }
    }
    if (audioPopover && !audioPopover.classList.contains('hidden')) {
      if (!audioPopover.contains(e.target) && !(eqBtn && eqBtn.contains(e.target))) {
        audioPopover.classList.add('hidden');
      }
    }
    if (shortcutsPopover && !shortcutsPopover.classList.contains('hidden')) {
      if (!shortcutsPopover.contains(e.target) && !(shortcutsHelpBtn && shortcutsHelpBtn.contains(e.target))) {
        shortcutsPopover.classList.add('hidden');
      }
    }
  });

  authBtn.addEventListener('click', async () => {
    if (state.loggedIn) {
      authPopover.classList.toggle('hidden');
      return;
    }
    showToast('Opening Google sign-in...');
    await window.backendAuthAction('login');
  });

  if (authReauthBtn) {
    authReauthBtn.addEventListener('click', async () => {
      authPopover.classList.add('hidden');
      showToast('Opening Google sign-in...');
      await window.backendAuthAction('login');
    });
  }

  if (authSwitchGoogleBtn) {
    authSwitchGoogleBtn.addEventListener('click', async () => {
      authPopover.classList.add('hidden');
      showToast('Opening Google sign-in...');
      await window.backendAuthAction('switch_google');
    });
  }

  if (authRefreshBtn) {
    authRefreshBtn.addEventListener('click', async () => {
      showToast('Syncing channels...');
      const auth = await window.backendAuthAction('refresh');
      applyAuthUI(auth);
      if (auth && auth.accounts && auth.accounts.length > 0) {
        showToast(`Synced ${auth.accounts.length} channel${auth.accounts.length === 1 ? '' : 's'}`);
      } else {
        showToast('Could not sync — try Refresh Sign-In');
      }
    });
  }

  authSignoutBtn.addEventListener('click', async () => {
    const auth = await window.backendAuthAction('logout');
    applyAuthUI(auth);
    showToast('Signed out');
  });

  authCloseBtn.addEventListener('click', () => {
    authPopover.classList.add('hidden');
  });

  quitBtn.addEventListener('click', async () => {
    await pushPlaybackState();
    window.backendQuitApp();
  });

  // Infinite Scroll Trigger on Playlist Bottom (and close track popover if list scrolls)
  playlistScroll.addEventListener('scroll', () => {
    if (addPlaylistPopover && !addPlaylistPopover.classList.contains('hidden')) {
      closeAddToPlaylistPopover();
    }
    if (state.activeTab !== 'queue' || state.filterQuery) return;
    if (state.isLoadingMore || state.playlist.length === 0) return;
    if (playlistScroll.scrollTop + playlistScroll.clientHeight >= playlistScroll.scrollHeight - 48) {
      triggerLoadMore();
    }
  });

  // Progress Bar Scrubbing
  progressRange.addEventListener('input', () => {
    state.isSeeking = true;
    const tr = state.playlist[state.currentIndex];
    const dur = Number.isFinite(media.duration) && media.duration > 0 ? media.duration : (tr ? tr.duration_sec : 0);
    const pct = parseFloat(progressRange.value) / 1000;
    progressFill.style.width = `${pct * 100}%`;
    timeElapsed.textContent = formatTime(pct * dur);
  });

  progressRange.addEventListener('change', () => {
    const tr = state.playlist[state.currentIndex];
    const dur = Number.isFinite(media.duration) && media.duration > 0 ? media.duration : (tr ? tr.duration_sec : 0);
    const pct = parseFloat(progressRange.value) / 1000;
    if (dur > 0) {
      if (Number.isFinite(media.duration) && media.duration > 0) {
        media.currentTime = pct * media.duration;
      } else {
        state.pendingSeekSec = pct * dur;
      }
    }
    state.isSeeking = false;
    pushPlaybackState();
  });

  progressRange.addEventListener('pointerup', () => {
    state.isSeeking = false;
  });
  progressRange.addEventListener('pointercancel', () => {
    state.isSeeking = false;
  });

  // Media Element Events
  media.addEventListener('loadedmetadata', () => {
    videoLoading.classList.add('hidden');
    media.playbackRate = state.playbackRate;
    if (state.pendingSeekSec > 0 && Number.isFinite(media.duration) && state.pendingSeekSec < media.duration) {
      media.currentTime = state.pendingSeekSec;
    }
    state.pendingSeekSec = 0;
    updateNowPlayingHeader();
  });

  media.addEventListener('canplay', () => {
    videoLoading.classList.add('hidden');
  });

  media.addEventListener('waiting', () => {
    stopPlaytimeTracker();
    videoLoading.classList.remove('hidden');
  });

  media.addEventListener('error', () => {
    stopPlaytimeTracker();
    videoLoading.classList.add('hidden');
    decayVuMeterToIdle();
    if (!media.getAttribute('src')) return;
    if (!state.userInitiatedPlay) {
      updatePlayButtonUI();
      return;
    }

    const seq = state.playbackSeq;
    state.consecutiveErrors++;
    if (state.consecutiveErrors >= 3) {
      state.userInitiatedPlay = false;
      updatePlayButtonUI();
      showToast('Playback paused: multiple stream errors');
      return;
    }

    showToast('Stream unavailable, skipping...');
    clearErrorSkipTimer();
    state.errorSkipTimer = setTimeout(() => {
      state.errorSkipTimer = null;
      if (seq !== state.playbackSeq) return;
      if (state.currentIndex + 1 < state.playlist.length || state.shuffle || state.repeatMode === 'all') {
        playNext(false);
      }
    }, 900);
  });

  media.addEventListener('play', () => {
    startVuMeterLoop();
  });

  media.addEventListener('playing', () => {
    state.consecutiveErrors = 0;
    state.userInitiatedPlay = true;
    clearErrorSkipTimer();
    videoLoading.classList.add('hidden');
    updatePlayButtonUI();
    startVuMeterLoop();
    startPlaytimeTracker();
    pushPlaybackState();
  });

  media.addEventListener('pause', () => {
    stopPlaytimeTracker();
    updatePlayButtonUI();
    decayVuMeterToIdle();
    pushPlaybackState();
  });

  media.addEventListener('ended', () => {
    stopPlaytimeTracker();
    updatePlayButtonUI();
    decayVuMeterToIdle();
    playNext(true);
  });

  media.addEventListener('timeupdate', () => {
    if (!media.paused && !media.ended) {
      flushSessionPlayTime();
      updatePlaytimeUI();
    }
    if (state.isSeeking) return;
    const tr = state.playlist[state.currentIndex];
    const dur = Number.isFinite(media.duration) && media.duration > 0 ? media.duration : (tr ? tr.duration_sec : 0);
    const cur = media.currentTime || 0;
    timeElapsed.textContent = formatTime(cur);
    timeTotal.textContent = formatTime(dur);

    if (dur > 0) {
      const pct = Math.min(100, Math.max(0, (cur / dur) * 100));
      progressFill.style.width = `${pct}%`;
      progressRange.value = String(Math.round(pct * 10));

      if (media.buffered && media.buffered.length > 0) {
        const bufEnd = media.buffered.end(media.buffered.length - 1);
        const bufPct = Math.min(100, (bufEnd / dur) * 100);
        progressBuffer.style.width = `${bufPct}%`;
      }
    }

    if (!media.paused && state.currentIndex >= 0 && Date.now() - state.lastSyncMs >= 1500) {
      pushPlaybackState();
    }
  });

  // In-window keyboard shortcuts
  document.addEventListener('keydown', async (e) => {
    if (e.key === 'Escape') {
      closeSearchDropdown();
      if (addPlaylistPopover && !addPlaylistPopover.classList.contains('hidden')) {
        closeAddToPlaylistPopover();
      }
      if (authPopover && !authPopover.classList.contains('hidden')) {
        authPopover.classList.add('hidden');
      }
      if (audioPopover && !audioPopover.classList.contains('hidden')) {
        audioPopover.classList.add('hidden');
      }
      if (shortcutsPopover && !shortcutsPopover.classList.contains('hidden')) {
        shortcutsPopover.classList.add('hidden');
      }
    }
    const tag = (document.activeElement && document.activeElement.tagName) || '';
    const isInput = tag === 'INPUT' || tag === 'TEXTAREA';
    if (isInput) {
      if (e.key === 'Escape') {
        document.activeElement.blur();
      }
      return;
    }

    switch (e.key) {
      case ' ':
        e.preventDefault();
        togglePlayPause();
        break;
      case 'ArrowLeft':
        e.preventDefault();
        if (Number.isFinite(media.duration)) {
          media.currentTime = Math.max(0, (media.currentTime || 0) - 5);
          pushPlaybackState();
        }
        break;
      case 'ArrowRight':
        e.preventDefault();
        if (Number.isFinite(media.duration)) {
          media.currentTime = Math.min(media.duration, (media.currentTime || 0) + 5);
          pushPlaybackState();
        }
        break;
      case 'ArrowUp':
        e.preventDefault();
        state.volume = Math.min(1, Math.round((state.volume + 0.05) * 100) / 100);
        state.muted = false;
        updateVolumeUI();
        pushPlaybackState();
        break;
      case 'ArrowDown':
        e.preventDefault();
        state.volume = Math.max(0, Math.round((state.volume - 0.05) * 100) / 100);
        updateVolumeUI();
        pushPlaybackState();
        break;
      case 'm':
      case 'M':
        state.muted = !state.muted;
        updateVolumeUI();
        pushPlaybackState();
        break;
      case 'v':
      case 'V': {
        const nextV = !state.videoEnabled;
        applyVideoModeUI(nextV);
        await window.backendToggleVideo(nextV);
        break;
      }
      case 'l':
      case 'L': {
        const nextL = !state.playlistExpanded;
        applyPlaylistModeUI(nextL);
        await window.backendTogglePlaylist(nextL);
        break;
      }
      case 's':
      case 'S':
        await toggleShuffle();
        break;
      case 'r':
      case 'R':
        await cycleRepeat();
        break;
      case 't':
      case 'T':
        await toggleTheme();
        break;
      case 'e':
      case 'E':
        if (eqBtn) eqBtn.click();
        break;
      case '?':
        if (shortcutsPopover) {
          if (shortcutsPopover.classList.contains('hidden')) {
            positionFloatingPopover(shortcutsPopover, eqBtn);
          } else {
            shortcutsPopover.classList.add('hidden');
          }
        }
        break;
    }
  });

  // Hooks invoked by Go backend (e.g. tray menu actions, sleep timer & auth completion)
  window.__trayCommand = (cmd) => {
    switch (cmd) {
      case 'play_pause':
        togglePlayPause();
        break;
      case 'next':
        playNext(false);
        break;
      case 'prev':
        playPrev();
        break;
      case 'toggle_video':
        applyVideoModeUI(!state.videoEnabled);
        break;
      case 'toggle_always_on_top':
        applyAlwaysOnTopUI(!state.alwaysOnTop);
        break;
      case 'toggle_theme':
        applyThemeUI(state.theme === 'light' ? 'dark' : 'light');
        break;
      case 'toggle_shuffle':
        state.shuffle = !state.shuffle;
        updateModesUI();
        showToast(state.shuffle ? 'Shuffle: On' : 'Shuffle: Off');
        break;
      case 'cycle_repeat':
        state.repeatMode = state.repeatMode === 'off' ? 'all' : state.repeatMode === 'all' ? 'one' : 'off';
        updateModesUI();
        showToast(state.repeatMode === 'one' ? 'Repeat: One' : state.repeatMode === 'all' ? 'Repeat: All' : 'Repeat: Off');
        break;
      case 'pause_sleep':
        if (!media.paused) {
          media.pause();
          updatePlayButtonUI();
          pushPlaybackState();
        }
        showToast('Sleep timer expired — paused');
        break;
      case 'system_suspend':
        stopPlaytimeTracker();
        if (!media.paused) {
          state.userInitiatedPlay = false;
          media.pause();
          updatePlayButtonUI();
          pushPlaybackState();
        }
        break;
      case 'system_resume':
        if (!media.paused && !media.ended) {
          startPlaytimeTracker();
        }
        break;
    }
  };

  window.__onStatsReset = () => {
    state.baseTotalPlaySec = 0;
    state.sessionPlayMs = 0;
    if (!media.paused && !media.ended) {
      state.playAnchorMs = performance.now();
    } else {
      state.playAnchorMs = 0;
    }
    updatePlaytimeUI();
    showToast('Listening stats cleared');
  };

  document.addEventListener('visibilitychange', () => {
    if (!document.hidden && !media.paused && !media.ended) {
      flushSessionPlayTime();
      updatePlaytimeUI();
    }
  });

  window.__onAuthUpdated = (auth, silent = false) => {
    applyAuthUI(auth);
    if (!silent && auth && auth.logged_in) {
      showToast(`Signed in as ${auth.account_name || auth.email || 'YouTube'}`);
    }
  };

  // Bootstrap initial state on load
  window.addEventListener('DOMContentLoaded', async () => {
    setupMediaSession();
    try {
      const init = await window.backendReady();
      if (!init) return;

      const s = init.state || {};
      state.playlist = Array.isArray(s.playlist) ? s.playlist : [];
      state.likedTracks = Array.isArray(s.liked_tracks) ? s.liked_tracks : [];
      state.savedPlaylists = Array.isArray(s.saved_playlists) ? s.saved_playlists : [];
      state.recentSearches = Array.isArray(s.recent_searches) ? s.recent_searches : [];
      state.currentIndex = typeof s.current_index === 'number' ? s.current_index : -1;
      state.volume = typeof s.volume === 'number' ? s.volume : 0.8;
      state.muted = Boolean(s.muted);
      state.shuffle = Boolean(s.shuffle);
      state.repeatMode = s.repeat_mode || 'off';
      state.vuStyle = s.vu_style === 'bar' ? 'bar' : 'led';
      state.eqPreset = ['flat', 'bass', 'vocal', 'bright'].includes(s.eq_preset) ? s.eq_preset : 'flat';
      state.playbackRate = (typeof s.playback_rate === 'number' && s.playback_rate >= 0.5 && s.playback_rate <= 2.0) ? s.playback_rate : 1.0;
      state.baseTotalPlaySec = (typeof s.total_play_time_sec === 'number' && s.total_play_time_sec > 0) ? Math.floor(s.total_play_time_sec) : 0;
      updatePlaytimeUI();

      if (vuMeter) {
        vuMeter.title = `Stereo VU Meter (${state.vuStyle === 'led' ? 'LED' : 'Bar'}) — Click to switch style`;
      }
      searchInput.value = s.search_query || '';
      applyVideoModeUI(Boolean(s.video_enabled));
      applyPlaylistModeUI(Boolean(s.playlist_expanded));
      applyAlwaysOnTopUI(Boolean(s.always_on_top));
      applyThemeUI(s.theme || 'dark');
      updateModesUI();
      updateVolumeUI();
      updateAudioSettingsUI();
      applyAuthUI(init.auth || {});
      renderActiveDrawerView(true);

      if (state.currentIndex >= 0 && state.currentIndex < state.playlist.length && init.stream_url) {
        state.pendingSeekSec = s.position_sec || 0;
        state.userInitiatedPlay = Boolean(s.was_playing);
        media.src = init.stream_url;
        media.playbackRate = state.playbackRate;
        media.load();
        if (s.was_playing) {
          media.play().catch(() => {
            state.userInitiatedPlay = false;
            updatePlayButtonUI();
          });
        } else {
          updatePlayButtonUI();
        }
        // Display saved position immediately before metadata finishes buffering.
        const tr = state.playlist[state.currentIndex];
        const dur = (tr && tr.duration_sec) || 0;
        if (dur > 0 && state.pendingSeekSec > 0) {
          const pct = Math.min(100, (state.pendingSeekSec / dur) * 100);
          progressFill.style.width = `${pct}%`;
          progressRange.value = String(Math.round(pct * 10));
          timeElapsed.textContent = formatTime(state.pendingSeekSec);
        }
      }
    } catch (err) {
      showToast('Startup init warning');
    } finally {
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          setTimeout(() => {
            if (typeof window.backendShowWindow === 'function') {
              window.backendShowWindow().catch(() => {});
            }
          }, 50);
        });
      });
    }
  });
})();
