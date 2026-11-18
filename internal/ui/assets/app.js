(() => {
  const media = document.getElementById('media-player');
  const videoPane = document.getElementById('video-pane');
  const videoLoading = document.getElementById('video-loading-overlay');
  const searchForm = document.getElementById('search-form');
  const searchInput = document.getElementById('search-input');
  const searchBtnIcon = document.getElementById('search-btn-icon');
  const searchSpinner = document.getElementById('search-spinner');

  const pinBtn = document.getElementById('pin-btn');
  const authBtn = document.getElementById('auth-btn');
  const authPopover = document.getElementById('auth-popover');
  const authAccountName = document.getElementById('auth-account-name');
  const authSignoutBtn = document.getElementById('auth-signout-btn');
  const authCloseBtn = document.getElementById('auth-close-btn');
  const quitBtn = document.getElementById('quit-btn');

  const currentThumb = document.getElementById('current-thumb');
  const thumbPlaceholder = document.getElementById('thumb-placeholder');
  const currentTitle = document.getElementById('current-title');
  const currentArtist = document.getElementById('current-artist');
  const trackCounter = document.getElementById('track-counter');

  const timeElapsed = document.getElementById('time-elapsed');
  const timeTotal = document.getElementById('time-total');
  const progressRange = document.getElementById('progress-range');
  const progressFill = document.getElementById('progress-fill');
  const progressBuffer = document.getElementById('progress-buffer');

  const prevBtn = document.getElementById('prev-btn');
  const playBtn = document.getElementById('play-btn');
  const nextBtn = document.getElementById('next-btn');
  const playIcon = document.getElementById('play-icon');
  const pauseIcon = document.getElementById('pause-icon');

  const muteBtn = document.getElementById('mute-btn');
  const volIcon = document.getElementById('vol-icon');
  const muteIcon = document.getElementById('mute-icon');
  const volumeRange = document.getElementById('volume-range');

  const videoToggleBtn = document.getElementById('video-toggle-btn');
  const playlistToggleBtn = document.getElementById('playlist-toggle-btn');

  const playlistDrawer = document.getElementById('playlist-drawer');
  const playlistList = document.getElementById('playlist-list');
  const playlistEmpty = document.getElementById('playlist-empty');
  const loadMoreFooter = document.getElementById('load-more-footer');
  const statusToast = document.getElementById('status-toast');

  const state = {
    playlist: [],
    currentIndex: -1,
    videoEnabled: false,
    playlistExpanded: true,
    alwaysOnTop: false,
    volume: 0.8,
    muted: false,
    isSeeking: false,
    isSearching: false,
    isLoadingMore: false,
    pendingSeekSec: 0,
    loggedIn: false,
    accountName: '',
    toastTimer: null,
  };

  function showToast(msg) {
    statusToast.textContent = msg;
    statusToast.classList.remove('hidden');
    if (state.toastTimer) clearTimeout(state.toastTimer);
    state.toastTimer = setTimeout(() => {
      statusToast.classList.add('hidden');
    }, 2600);
  }

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

  function updatePlayButtonUI() {
    const playing = !media.paused && !media.ended;
    playIcon.classList.toggle('hidden', playing);
    pauseIcon.classList.toggle('hidden', !playing);
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
  }

  function applyAlwaysOnTopUI(enabled) {
    state.alwaysOnTop = enabled;
    pinBtn.classList.toggle('active', enabled);
    pinBtn.title = enabled ? 'Stay on top: On' : 'Stay on top: Off';
  }

  function applyAuthUI(auth) {
    state.loggedIn = Boolean(auth && auth.logged_in);
    state.accountName = (auth && auth.account_name) || 'YouTube Account';
    authBtn.classList.toggle('signed-in', state.loggedIn);
    authBtn.title = state.loggedIn ? `Signed in as ${state.accountName}` : 'Sign in with Google';
    authAccountName.textContent = state.accountName;
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
      }
      const dur = Number.isFinite(media.duration) && media.duration > 0 ? media.duration : (tr.duration_sec || 0);
      timeTotal.textContent = formatTime(dur);
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
    }
  }

  function renderPlaylist(scrollActiveIntoView = false) {
    playlistList.innerHTML = '';
    playlistEmpty.classList.toggle('hidden', state.playlist.length > 0);

    const frag = document.createDocumentFragment();
    state.playlist.forEach((tr, idx) => {
      frag.appendChild(createTrackRow(tr, idx));
    });
    playlistList.appendChild(frag);

    if (scrollActiveIntoView && state.currentIndex >= 0) {
      const activeEl = playlistList.children[state.currentIndex];
      if (activeEl) {
        activeEl.scrollIntoView({ block: 'nearest' });
      }
    }
    updateNowPlayingHeader();
  }

  function appendPlaylistItems(newTracks, startIndex) {
    playlistEmpty.classList.toggle('hidden', state.playlist.length > 0);
    const frag = document.createDocumentFragment();
    newTracks.forEach((tr, offset) => {
      frag.appendChild(createTrackRow(tr, startIndex + offset));
    });
    playlistList.appendChild(frag);
    updateNowPlayingHeader();
  }

  function createTrackRow(tr, idx) {
    const row = document.createElement('div');
    row.className = 'track-item' + (idx === state.currentIndex ? ' active' : '');
    row.dataset.index = String(idx);

    const idxEl = document.createElement('div');
    idxEl.className = 'track-idx';
    idxEl.textContent = idx === state.currentIndex ? '▶' : String(idx + 1);

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

    row.appendChild(idxEl);
    row.appendChild(img);
    row.appendChild(info);
    row.appendChild(dur);

    row.addEventListener('click', () => {
      playTrackAt(idx, true);
    });
    return row;
  }

  function updateActiveRowHighlight() {
    const children = playlistList.children;
    for (let i = 0; i < children.length; i++) {
      const el = children[i];
      const active = i === state.currentIndex;
      el.classList.toggle('active', active);
      const idxEl = el.querySelector('.track-idx');
      if (idxEl) idxEl.textContent = active ? '▶' : String(i + 1);
    }
    const activeEl = children[state.currentIndex];
    if (activeEl) {
      activeEl.scrollIntoView({ block: 'nearest' });
    }
  }

  async function playTrackAt(index, autoplay = true, startPosSec = 0) {
    if (index < 0 || index >= state.playlist.length) return;
    state.currentIndex = index;
    state.pendingSeekSec = startPosSec;
    updateActiveRowHighlight();
    updateNowPlayingHeader();

    videoLoading.classList.remove('hidden');
    try {
      const streamURL = await window.backendSelectTrack(index);
      if (!streamURL) {
        videoLoading.classList.add('hidden');
        return;
      }
      media.src = streamURL;
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
      media.play().catch(() => {});
    } else {
      media.pause();
    }
  }

  function playNext() {
    if (state.playlist.length === 0) return;
    const next = state.currentIndex + 1;
    if (next < state.playlist.length) {
      playTrackAt(next, true);
      if (next >= state.playlist.length - 3) {
        triggerLoadMore();
      }
    }
  }

  function playPrev() {
    if (state.playlist.length === 0) return;
    if (media.currentTime > 3) {
      media.currentTime = 0;
      pushPlaybackState();
      return;
    }
    const prev = state.currentIndex - 1;
    if (prev >= 0) {
      playTrackAt(prev, true);
    }
  }

  async function triggerLoadMore() {
    if (state.isLoadingMore || state.playlist.length === 0) return;
    state.isLoadingMore = true;
    loadMoreFooter.classList.remove('hidden');
    try {
      const res = await window.backendLoadMore();
      if (res && Array.isArray(res.added) && res.added.length > 0) {
        const startIdx = state.playlist.length;
        state.playlist.push(...res.added);
        appendPlaylistItems(res.added, startIdx);
        showToast(`Added ${res.added.length} more songs`);
      }
    } catch (_) {
      // ignore transient continuation error
    } finally {
      state.isLoadingMore = false;
      loadMoreFooter.classList.add('hidden');
    }
  }

  function pushPlaybackState() {
    if (typeof window.backendUpdatePlaybackState !== 'function') return;
    const pos = Number.isFinite(media.currentTime) ? media.currentTime : 0;
    const playing = !media.paused && !media.ended;
    window.backendUpdatePlaybackState(
      state.currentIndex,
      pos,
      playing,
      state.volume,
      state.muted
    ).catch(() => {});
  }

  // --- Event Listeners ---
  searchForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const q = searchInput.value.trim();
    if (!q || state.isSearching) return;

    state.isSearching = true;
    searchBtnIcon.classList.add('hidden');
    searchSpinner.classList.remove('hidden');
    try {
      const res = await window.backendSearch(q);
      if (!res || !Array.isArray(res.playlist) || res.playlist.length === 0) {
        showToast('No playable songs found');
        return;
      }
      state.playlist = res.playlist;
      state.currentIndex = 0;
      renderPlaylist(true);
      showToast(`Built ${state.playlist.length}-song playlist`);
      await playTrackAt(0, true, 0);
    } catch (err) {
      showToast('Search failed: ' + (err || 'network error'));
    } finally {
      state.isSearching = false;
      searchBtnIcon.classList.remove('hidden');
      searchSpinner.classList.add('hidden');
    }
  });

  playBtn.addEventListener('click', togglePlayPause);
  nextBtn.addEventListener('click', playNext);
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

  authBtn.addEventListener('click', async () => {
    if (state.loggedIn) {
      authPopover.classList.toggle('hidden');
      return;
    }
    showToast('Opening Google sign-in...');
    await window.backendAuthAction('login');
  });

  authSignoutBtn.addEventListener('click', async () => {
    const auth = await window.backendAuthAction('logout');
    applyAuthUI(auth);
    showToast('Signed out');
  });

  authCloseBtn.addEventListener('click', () => {
    authPopover.classList.add('hidden');
  });

  quitBtn.addEventListener('click', () => {
    pushPlaybackState();
    window.backendQuitApp();
  });

  // Infinite Scroll Trigger on Playlist Bottom
  playlistDrawer.addEventListener('scroll', () => {
    if (state.isLoadingMore || state.playlist.length === 0) return;
    if (playlistDrawer.scrollTop + playlistDrawer.clientHeight >= playlistDrawer.scrollHeight - 48) {
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
    if (dur > 0 && Number.isFinite(media.duration)) {
      media.currentTime = pct * media.duration;
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
    videoLoading.classList.remove('hidden');
  });

  media.addEventListener('error', () => {
    videoLoading.classList.add('hidden');
    if (media.getAttribute('src')) {
      showToast('Stream unavailable, skipping...');
      setTimeout(() => {
        if (state.currentIndex + 1 < state.playlist.length) {
          playNext();
        }
      }, 900);
    }
  });

  media.addEventListener('playing', () => {
    videoLoading.classList.add('hidden');
    updatePlayButtonUI();
    pushPlaybackState();
  });

  media.addEventListener('pause', () => {
    updatePlayButtonUI();
    pushPlaybackState();
  });

  media.addEventListener('ended', () => {
    updatePlayButtonUI();
    playNext();
  });

  media.addEventListener('timeupdate', () => {
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
  });

  // Periodic state persistence heartbeat while playing
  setInterval(() => {
    if (!media.paused && state.currentIndex >= 0) {
      pushPlaybackState();
    }
  }, 3000);

  // Hooks invoked by Go backend (e.g. tray menu actions & auth login completion)
  window.__trayCommand = (cmd) => {
    switch (cmd) {
      case 'play_pause':
        togglePlayPause();
        break;
      case 'next':
        playNext();
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
    }
  };

  window.__onAuthUpdated = (auth) => {
    applyAuthUI(auth);
    if (auth && auth.logged_in) {
      showToast(`Signed in as ${auth.account_name || 'YouTube'}`);
    }
  };

  // Bootstrap initial state on load
  window.addEventListener('DOMContentLoaded', async () => {
    try {
      const init = await window.backendReady();
      if (!init) return;

      const s = init.state || {};
      state.playlist = Array.isArray(s.playlist) ? s.playlist : [];
      state.currentIndex = typeof s.current_index === 'number' ? s.current_index : -1;
      state.volume = typeof s.volume === 'number' ? s.volume : 0.8;
      state.muted = Boolean(s.muted);

      searchInput.value = s.search_query || '';
      applyVideoModeUI(Boolean(s.video_enabled));
      applyPlaylistModeUI(Boolean(s.playlist_expanded));
      applyAlwaysOnTopUI(Boolean(s.always_on_top));
      updateVolumeUI();
      applyAuthUI(init.auth || {});
      renderPlaylist(true);

      if (state.currentIndex >= 0 && state.currentIndex < state.playlist.length && init.stream_url) {
        state.pendingSeekSec = s.position_sec || 0;
        media.src = init.stream_url;
        media.load();
        if (s.was_playing) {
          media.play().catch(() => updatePlayButtonUI());
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
    }
  });
})();
