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
  const likeBtn = document.getElementById('like-btn');
  const heartOutlineIcon = document.getElementById('heart-outline-icon');
  const heartFilledIcon = document.getElementById('heart-filled-icon');
  const trackCounter = document.getElementById('track-counter');

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

  const videoToggleBtn = document.getElementById('video-toggle-btn');
  const playlistToggleBtn = document.getElementById('playlist-toggle-btn');

  const playlistDrawer = document.getElementById('playlist-drawer');
  const playlistScroll = document.getElementById('playlist-scroll');
  const playlistList = document.getElementById('playlist-list');
  const playlistEmpty = document.getElementById('playlist-empty');
  const loadMoreFooter = document.getElementById('load-more-footer');
  const statusToast = document.getElementById('status-toast');

  const tabQueue = document.getElementById('tab-queue');
  const tabLiked = document.getElementById('tab-liked');
  const tabSaved = document.getElementById('tab-saved');
  const saveQueueBtn = document.getElementById('save-queue-btn');
  const saveQueueBar = document.getElementById('save-queue-bar');
  const saveQueueInput = document.getElementById('save-queue-input');
  const saveQueueConfirm = document.getElementById('save-queue-confirm');
  const saveQueueCancel = document.getElementById('save-queue-cancel');
  const filterRow = document.getElementById('filter-row');
  const playlistFilter = document.getElementById('playlist-filter');
  const playLikedBtn = document.getElementById('play-liked-btn');

  const state = {
    playlist: [],
    likedTracks: [],
    savedPlaylists: [],
    currentIndex: -1,
    videoEnabled: false,
    playlistExpanded: true,
    alwaysOnTop: false,
    shuffle: false,
    repeatMode: 'off', // 'off' | 'all' | 'one'
    shuffleHistory: [],
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

  function updateTabsUI() {
    tabQueue.classList.toggle('active', state.activeTab === 'queue');
    tabLiked.classList.toggle('active', state.activeTab === 'liked');
    tabSaved.classList.toggle('active', state.activeTab === 'saved');

    tabQueue.textContent = state.playlist.length > 0 ? `Queue (${state.playlist.length})` : 'Queue';
    tabLiked.textContent = state.likedTracks.length > 0 ? `Liked (${state.likedTracks.length})` : 'Liked ♥';
    tabSaved.textContent = state.savedPlaylists.length > 0 ? `Saved (${state.savedPlaylists.length})` : 'Saved';

    filterRow.classList.toggle('hidden', state.activeTab === 'saved');
    playLikedBtn.classList.toggle('hidden', state.activeTab !== 'liked' || state.likedTracks.length === 0);
    saveQueueBtn.classList.toggle('hidden', state.playlist.length === 0);
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

  function createQueueRow(tr, idx) {
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

    const actions = document.createElement('div');
    actions.className = 'track-actions';

    const rowLike = document.createElement('button');
    rowLike.type = 'button';
    const liked = isTrackLiked(tr.id);
    rowLike.className = 'row-act-btn' + (liked ? ' liked' : '');
    rowLike.title = liked ? 'Unlike' : 'Like';
    rowLike.textContent = liked ? '♥' : '♡';
    rowLike.addEventListener('click', async (e) => {
      e.stopPropagation();
      await toggleTrackLike(tr);
    });

    const rowNext = document.createElement('button');
    rowNext.type = 'button';
    rowNext.className = 'row-act-btn';
    rowNext.title = 'Play next';
    rowNext.textContent = '⤵';
    rowNext.addEventListener('click', async (e) => {
      e.stopPropagation();
      await moveTrackPlayNext(idx);
    });

    const rowRemove = document.createElement('button');
    rowRemove.type = 'button';
    rowRemove.className = 'row-act-btn danger';
    rowRemove.title = 'Remove from queue';
    rowRemove.textContent = '×';
    rowRemove.addEventListener('click', async (e) => {
      e.stopPropagation();
      await removeTrackFromQueue(idx);
    });

    actions.appendChild(rowLike);
    actions.appendChild(rowNext);
    actions.appendChild(rowRemove);

    row.appendChild(idxEl);
    row.appendChild(img);
    row.appendChild(info);
    row.appendChild(dur);
    row.appendChild(actions);

    row.addEventListener('click', () => {
      playTrackAt(idx, true);
    });
    return row;
  }

  function createLikedRow(tr, idx) {
    const row = document.createElement('div');
    const isPlayingThis = state.currentIndex >= 0 && state.playlist[state.currentIndex]?.id === tr.id;
    row.className = 'track-item' + (isPlayingThis ? ' active' : '');

    const idxEl = document.createElement('div');
    idxEl.className = 'track-idx';
    idxEl.textContent = isPlayingThis ? '▶' : String(idx + 1);

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

    const rowUnlike = document.createElement('button');
    rowUnlike.type = 'button';
    rowUnlike.className = 'row-act-btn liked';
    rowUnlike.title = 'Remove from Liked';
    rowUnlike.textContent = '♥';
    rowUnlike.addEventListener('click', async (e) => {
      e.stopPropagation();
      await toggleTrackLike(tr);
    });

    actions.appendChild(rowUnlike);

    row.appendChild(idxEl);
    row.appendChild(img);
    row.appendChild(info);
    row.appendChild(dur);
    row.appendChild(actions);

    row.addEventListener('click', async () => {
      await playTrackFromLiked(tr);
    });
    return row;
  }

  function renderSavedPlaylists() {
    playlistEmpty.textContent = 'No saved playlists yet. Click "+ Save" to store your current queue.';
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

      card.appendChild(meta);
      card.appendChild(delBtn);

      card.addEventListener('click', async () => {
        try {
          const res = await window.backendLoadSavedPlaylist(sp.name);
          if (!res || !Array.isArray(res.playlist) || res.playlist.length === 0) return;
          state.playlist = res.playlist;
          state.shuffleHistory = [];
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
      if (state.activeTab === 'liked') renderActiveDrawerView(false);
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
    } catch (_) {}
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
      clearErrorSkipTimer();
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
      renderActiveDrawerView(false);
      await window.backendUpdatePlaylist(state.playlist, nextIdx).catch(() => {});
      await playTrackAt(nextIdx, true);
      return;
    }

    renderActiveDrawerView(false);
    await window.backendUpdatePlaylist(state.playlist, state.currentIndex).catch(() => {});
  }

  async function playTrackFromLiked(tr) {
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
      media.play().catch(() => {});
    } else {
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
      let nextIdx = 0;
      if (state.playlist.length > 1) {
        do {
          nextIdx = Math.floor(Math.random() * state.playlist.length);
        } while (nextIdx === state.currentIndex);
      }
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
    const pos = Number.isFinite(media.currentTime) ? media.currentTime : 0;
    const playing = !media.paused && !media.ended;
    state.lastSyncMs = Date.now();
    return window.backendUpdatePlaybackState(
      state.currentIndex,
      pos,
      playing,
      state.volume,
      state.muted
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

  // --- Event Listeners ---
  searchForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const q = searchInput.value.trim();
    if (!q || state.isSearching) return;

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
      state.shuffleHistory = [];
      state.consecutiveErrors = 0;
      state.currentIndex = 0;
      state.activeTab = 'queue';
      state.filterQuery = '';
      playlistFilter.value = '';
      renderActiveDrawerView(true);
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

  tabQueue.addEventListener('click', () => {
    state.activeTab = 'queue';
    renderActiveDrawerView(true);
  });
  tabLiked.addEventListener('click', () => {
    state.activeTab = 'liked';
    renderActiveDrawerView(false);
  });
  tabSaved.addEventListener('click', () => {
    state.activeTab = 'saved';
    renderActiveDrawerView(false);
  });

  playlistFilter.addEventListener('input', () => {
    state.filterQuery = playlistFilter.value.trim();
    renderActiveDrawerView(false);
  });

  playLikedBtn.addEventListener('click', async () => {
    if (state.likedTracks.length === 0) return;
    state.playlist = state.likedTracks.map((t) => ({ ...t }));
    state.shuffleHistory = [];
    state.currentIndex = 0;
    state.activeTab = 'queue';
    renderActiveDrawerView(true);
    await window.backendUpdatePlaylist(state.playlist, 0).catch(() => {});
    await playTrackAt(0, true);
  });

  saveQueueBtn.addEventListener('click', () => {
    saveQueueBar.classList.toggle('hidden');
    if (!saveQueueBar.classList.contains('hidden')) {
      saveQueueInput.value = searchInput.value.trim() || `Playlist ${state.savedPlaylists.length + 1}`;
      saveQueueInput.focus();
      saveQueueInput.select();
    }
  });

  saveQueueCancel.addEventListener('click', () => {
    saveQueueBar.classList.add('hidden');
  });

  async function confirmSaveQueue() {
    const name = saveQueueInput.value.trim();
    if (!name) return;
    try {
      const updated = await window.backendSavePlaylist(name);
      state.savedPlaylists = Array.isArray(updated) ? updated : [];
      saveQueueBar.classList.add('hidden');
      renderActiveDrawerView(false);
      showToast(`Saved "${name}"`);
    } catch (err) {
      showToast('Failed to save playlist');
    }
  }

  saveQueueConfirm.addEventListener('click', confirmSaveQueue);
  saveQueueInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      confirmSaveQueue();
    } else if (e.key === 'Escape') {
      saveQueueBar.classList.add('hidden');
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

  // Infinite Scroll Trigger on Playlist Bottom
  playlistScroll.addEventListener('scroll', () => {
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
    if (!media.getAttribute('src')) return;

    const seq = state.playbackSeq;
    state.consecutiveErrors++;
    if (state.consecutiveErrors >= 3) {
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

  media.addEventListener('playing', () => {
    state.consecutiveErrors = 0;
    clearErrorSkipTimer();
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
    playNext(true);
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

    if (!media.paused && state.currentIndex >= 0 && Date.now() - state.lastSyncMs >= 1500) {
      pushPlaybackState();
    }
  });

  // In-window keyboard shortcuts
  document.addEventListener('keydown', async (e) => {
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
    setupMediaSession();
    try {
      const init = await window.backendReady();
      if (!init) return;

      const s = init.state || {};
      state.playlist = Array.isArray(s.playlist) ? s.playlist : [];
      state.likedTracks = Array.isArray(s.liked_tracks) ? s.liked_tracks : [];
      state.savedPlaylists = Array.isArray(s.saved_playlists) ? s.saved_playlists : [];
      state.currentIndex = typeof s.current_index === 'number' ? s.current_index : -1;
      state.volume = typeof s.volume === 'number' ? s.volume : 0.8;
      state.muted = Boolean(s.muted);
      state.shuffle = Boolean(s.shuffle);
      state.repeatMode = s.repeat_mode || 'off';

      searchInput.value = s.search_query || '';
      applyVideoModeUI(Boolean(s.video_enabled));
      applyPlaylistModeUI(Boolean(s.playlist_expanded));
      applyAlwaysOnTopUI(Boolean(s.always_on_top));
      updateModesUI();
      updateVolumeUI();
      applyAuthUI(init.auth || {});
      renderActiveDrawerView(true);

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
