<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte';
  import { auth } from '../lib/auth.svelte';

  interface KifuItem {
    id: string;
    title: string;
    black_player?: string;
    black_rank?: string;
    white_player?: string;
    white_rank?: string;
    result?: string;
    game_date?: string;
    created_at: string;
    has_ogp?: boolean;
  }

  const dispatch = createEventDispatcher<{
    selectKifu: string;
    createKifu: void;
  }>();

  let { userId = "" } = $props<{
    userId?: string;
  }>();

  let publicMode = $derived(!!userId);

  // Reactive states using Svelte 5 Runes
  let kifus = $state.raw<KifuItem[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let ownerUsername = $state<string | null>(null);

  // Form states
  let title = $state("");
  let sgfData = $state("");
  let showUploadForm = $state(false);
  let uploading = $state(false);

  // Filter states
  let searchQuery = $state("");
  let startDate = $state("");
  let endDate = $state("");

  // Derived filtered kifu list
  let filteredKifus = $derived.by(() => {
    return kifus.filter(k => {
      // 1. Text search
      if (searchQuery.trim()) {
        const query = searchQuery.toLowerCase();
        const titleMatch = k.title?.toLowerCase().includes(query);
        const blackMatch = k.black_player?.toLowerCase().includes(query);
        const whiteMatch = k.white_player?.toLowerCase().includes(query);
        if (!titleMatch && !blackMatch && !whiteMatch) {
          return false;
        }
      }

      // 2. Date range filtering (game_date is YYYY-MM-DD)
      const gameDateStr = k.game_date;
      if (gameDateStr) {
        if (startDate && gameDateStr < startDate) return false;
        if (endDate && gameDateStr > endDate) return false;
      } else {
        // Exclude if filter is active but game_date is missing
        if (startDate || endDate) return false;
      }

      return true;
    });
  });

  // Type helper for Materialize global object
  const getM = () => (window as any).M;

  async function fetchKifus(): Promise<void> {
    loading = true;
    error = null; // 前回のエラーをクリア
    try {
      const url = publicMode ? `/api/u/${userId}/kifus` : '/api/kifus';
      const headers = publicMode ? {} : auth.getHeaders();
      const res = await fetch(url, { headers });
      if (!res.ok) {
        if (res.status === 401) {
          // トークンが無効・期限切れの場合は再ログイン促す
          throw new Error("認証エラー: 再ログインしてください");
        }
        throw new Error(`棋譜の取得に失敗しました (${res.status})`);
      }
      kifus = await res.json();
    } catch (err: any) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  async function handleUpload(): Promise<void> {
    if (!sgfData.trim()) return;

    uploading = true;
    try {
      const res = await fetch('/api/kifus', {
        method: 'POST',
        headers: auth.getHeaders(),
        body: JSON.stringify({
          title: title.trim() || undefined,
          sgf_data: sgfData.trim()
        })
      });

      if (!res.ok) {
        const errJson = await res.json();
        throw new Error(errJson.error || "Failed to upload game");
      }

      // Reset form
      title = "";
      sgfData = "";
      showUploadForm = false;
      
      // Reload games
      await fetchKifus();
      
      const M = getM();
      if (M) {
        M.toast({ html: '棋譜が登録されました！', classes: 'green darken-1' });
      }
    } catch (err: any) {
      const M = getM();
      if (M) {
        M.toast({ html: 'エラー: ' + err.message, classes: 'red darken-1' });
      }
    } finally {
      uploading = false;
    }
  }

  async function handleDelete(id: string, e: MouseEvent): Promise<void> {
    e.stopPropagation(); // Avoid triggering card click (selecting kifu)
    if (!confirm("本当にこの棋譜を削除しますか？")) return;

    try {
      const res = await fetch(`/api/kifus/${id}`, {
        method: 'DELETE',
        headers: auth.getHeaders()
      });
      if (!res.ok) throw new Error("Failed to delete game");
      
      kifus = kifus.filter(k => k.id !== id);
      const M = getM();
      if (M) {
        M.toast({ html: '棋譜が削除されました', classes: 'grey darken-3' });
      }
    } catch (err: any) {
      const M = getM();
      if (M) {
        M.toast({ html: 'エラー: ' + err.message, classes: 'red' });
      }
    }
  }

  // Handle local SGF file upload
  function handleFileChange(event: Event): void {
    const target = event.target as HTMLInputElement;
    const file = target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (e: ProgressEvent<FileReader>) => {
      const result = e.target?.result;
      if (typeof result === 'string') {
        sgfData = result;
        if (!title) {
          // Use filename without extension as default title
          title = file.name.replace(/\.sgf$/i, '');
        }
      }
    };
    reader.readAsText(file);
  }

  async function fetchOwnerUsername(): Promise<void> {
    if (!publicMode || !userId) return;
    try {
      const res = await fetch(`/api/users/${userId}/username`);
      if (res.ok) {
        const data = await res.json();
        ownerUsername = data.username;
      }
    } catch (err) {
      console.error("Failed to fetch owner username:", err);
    }
  }

  let isDrawerOpen = $state(false);

  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'Escape' && isDrawerOpen) {
      isDrawerOpen = false;
    }
  }

  onMount(() => {
    fetchKifus();
    fetchOwnerUsername();
  });
</script>
<svelte:window onkeydown={handleKeyDown} />

<!-- Floating Vogue Drawer Toggle Button (Left edge vertical ribbon) -->
<button 
  onclick={() => isDrawerOpen = !isDrawerOpen} 
  class="em-vogue-drawer-toggle em-float-badge"
  style="position: fixed; left: 0; top: 25%; z-index: 1000; border-radius: 0px !important; border: 1.5px solid var(--wc-text) !important; border-left: none !important; box-shadow: 3px 3px 0px var(--wc-text); background: var(--wc-accent-warm) !important; color: var(--wc-text) !important; padding: 12px 6px; writing-mode: vertical-rl; text-orientation: mixed; font-family: 'JetBrains Mono', monospace; font-size: 0.72rem; font-weight: bold; letter-spacing: 0.15em; cursor: pointer; text-transform: uppercase;"
>
  [ VIEW FILTERS & OVERVIEW ]
</button>

{#if isDrawerOpen}
  <div 
    class="em-vogue-overlay animate-fade-in" 
    onclick={() => isDrawerOpen = false}
    style="position: fixed; top: 0; left: 0; width: 100vw; height: 100vh; background: rgba(30,42,38,0.4); backdrop-filter: blur(4px); z-index: 998; cursor: pointer;"
    tabindex="-1"
    aria-hidden="true"
  ></div>
{/if}

<!-- Vogue Editorial Drawer Panel -->
<div 
  class="em-vogue-drawer {isDrawerOpen ? 'open' : ''}"
  style="position: fixed; top: 0; left: 0; height: 100vh; background: var(--wc-surface); border-right: 2px solid var(--wc-text); z-index: 999; transform: translateX({isDrawerOpen ? '0' : '-100%'}); transition: transform 0.4s cubic-bezier(0.25, 0.46, 0.45, 0.94); padding: 40px 24px; overflow-y: auto; box-shadow: 10px 0 30px rgba(0,0,0,0.15); text-align: left;"
  role="dialog"
  aria-modal="true"
  aria-label="フィルターと概要"
>
  <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 30px; border-bottom: 2px solid var(--wc-text); padding-bottom: 12px;">
    <h2 style="font-family: 'Cormorant Garamond', serif; font-size: 1.6rem; font-weight: 800; margin: 0; color: var(--wc-text); text-transform: uppercase; letter-spacing: 0.08em;">
      the chronicle.
    </h2>
    <button type="button" onclick={() => isDrawerOpen = false} aria-label="フィルターを閉じる" style="background: transparent; border: none; cursor: pointer; color: var(--wc-text); display: flex; align-items: center; justify-content: center;">
      <i class="material-icons" aria-hidden="true" style="font-size: 1.5rem;">close</i>
    </button>
  </div>

  <!-- Editorial Note (社説 - 巨大なドロップキャップ) -->
  <div style="font-family: 'Shippori Mincho B1', 'Noto Serif JP', serif; font-size: 0.82rem; line-height: 1.9; color: var(--wc-text); margin-bottom: 24px; text-align: justify; text-justify: inter-word; opacity: 0.95;">
    <span style="float: left; font-size: 3.2rem; font-family: 'Cormorant Garamond', serif; line-height: 0.8; margin-top: 4px; margin-right: 8px; font-weight: 700; color: var(--wc-accent); text-transform: uppercase;">G</span>o is not merely a game of patterns, but a silent conversation recorded in stones. Here lies a personal library, structured through vintage print aesthetics to preserve each tactical path with clarity and editorial elegance.
  </div>

  <!-- Actions (Newspaper Buttons) -->
  {#if !publicMode}
    <div style="display: flex; flex-direction: column; gap: 12px; margin-bottom: 24px; border-bottom: 1.5px solid var(--wc-text); padding-bottom: 24px;">
      <button class="nm-btn-primary" onclick={() => { dispatch('createKifu'); isDrawerOpen = false; }} style="width: 100%; border-radius: 0px !important; border: 1.5px solid var(--wc-text) !important; box-shadow: 3px 3px 0px var(--wc-text) !important;">
        <i class="material-icons" style="font-size: 1.1rem; margin-right: 6px; vertical-align: middle;">edit</i>自分で棋譜を作成
      </button>
      <button class="nm-btn" onclick={() => showUploadForm = !showUploadForm} style="width: 100%; border-radius: 0px !important; background: var(--wc-surface) !important; border: 1.5px solid var(--wc-text) !important; box-shadow: 3px 3px 0px var(--wc-text) !important; color: var(--wc-text) !important;">
        <i class="material-icons" style="font-size: 1.1rem; margin-right: 6px; vertical-align: middle;">{showUploadForm ? 'close' : 'cloud_upload'}</i>
        {showUploadForm ? '閉じる' : 'SGFファイルをアップロード'}
      </button>
    </div>
  {/if}

  <!-- Filtering Panel -->
  {#if kifus.length > 0}
    <div style="border-bottom: 1.5px solid var(--wc-text); padding-bottom: 24px; margin-bottom: 24px;">
      <h3 style="font-family: 'Cormorant Garamond', serif; font-size: 1.1rem; font-weight: 700; text-transform: uppercase; letter-spacing: 0.1em; margin: 0 0 16px 0; color: var(--wc-text); border-bottom: 1px solid var(--wc-text); padding-bottom: 4px; display: inline-block;">Filter Records</h3>
      
      <div style="display: flex; flex-direction: column; gap: 14px;">
        <div class="input-field" style="margin: 0; position: relative;">
          <input id="search-query" type="text" class="nm-input" bind:value={searchQuery} placeholder="タイトル・対局者" style="margin-bottom: 0; border-radius: 0 !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important;" />
          <label for="search-query" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text); font-size: 0.8rem; font-weight: 600;">キーワード</label>
        </div>
        <div class="input-field" style="margin: 0; position: relative;">
          <input id="start-date" type="date" class="nm-input" bind:value={startDate} style="margin-bottom: 0; border-radius: 0 !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important;" />
          <label for="start-date" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text); font-size: 0.8rem; font-weight: 600;">開始日</label>
        </div>
        <div class="input-field" style="margin: 0; position: relative;">
          <input id="end-date" type="date" class="nm-input" bind:value={endDate} style="margin-bottom: 0; border-radius: 0 !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important;" />
          <label for="end-date" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text); font-size: 0.8rem; font-weight: 600;">終了日</label>
        </div>
        {#if searchQuery || startDate || endDate}
          <button type="button" class="clear-filter-btn" onclick={() => { searchQuery = ""; startDate = ""; endDate = ""; }} style="align-self: flex-end; padding: 4px 10px; font-size: 0.75rem; display: inline-flex; align-items: center; gap: 4px; font-family: 'JetBrains Mono', sans-serif; border: 1.5px solid var(--wc-text); background: var(--wc-surface); box-shadow: 2px 2px 0px var(--wc-text); color: var(--wc-text); cursor: pointer; border-radius: 0;">
            <i class="material-icons" aria-hidden="true" style="font-size: 0.85rem;">clear_all</i>条件クリア
          </button>
        {/if}
      </div>
    </div>
  {/if}

  <!-- Vertical Japanese Column -->
  <div style="display: flex; justify-content: flex-end; padding-top: 10px; height: 160px;">
    <div class="em-newspaper-vertical-col" style="opacity: 0.8; height: 140px; border-left-color: var(--wc-text);">
      一打一打に宿る思考を印刷インクの温もりとともに残す。静かなる黒と白の調和。
    </div>
  </div>
</div>

<div class="row" style="margin-top: 1.5rem; position: relative;">
  <!-- Right Side: Archive Column (Portfolio Column 2) -->
  <div class="col s12 m10 offset-m1 l8 offset-l2" style="position: relative; z-index: 2; margin-top: 2rem;">
    <!-- Upload Form Area (if open) -->
    {#if showUploadForm}
      <div class="row" style="margin-bottom: 2rem;">
        <div class="col s12">
          <div class="em-portfolio-section" style="border-color: var(--wc-text) !important; padding: 28px 24px 20px 24px !important; background: var(--wc-surface) !important; box-shadow: 6px 6px 0px var(--wc-text) !important;">
            <!-- Overlap Badge -->
            <span class="em-collage-tag-pastel em-float-badge" style="position: absolute; top: -14px; left: 16px; font-size: 0.72rem; box-shadow: 2px 2px 0px var(--wc-text);">
              UPLOADER STUDIO
            </span>

            <div class="card-content" style="padding: 12px 0 0 0;">
              <div class="row" style="margin-bottom: 0; display: flex; flex-wrap: wrap; gap: 14px 0;">
                <div class="file-field input-field col s12 m6" style="margin-top: 0; margin-bottom: 0; display: flex; gap: 10px; align-items: center;">
                  <div class="nm-btn" style="position: relative; overflow: hidden; white-space: nowrap; border-radius: 0 !important; border: 1.5px solid var(--wc-text) !important; box-shadow: 2.5px 2.5px 0px var(--wc-text); font-weight: bold; background: var(--wc-surface) !important; color: var(--wc-text) !important;">
                    <span>SGFファイル選択</span>
                    <input type="file" accept=".sgf" onchange={handleFileChange} />
                  </div>
                  <div class="file-path-wrapper" style="flex-grow: 1; padding-left: 0;">
                    <input class="file-path validate nm-input" type="text" placeholder="または以下に直接貼り付け" style="margin-bottom: 0; border-radius: 0 !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important;" />
                  </div>
                </div>

                <div class="input-field col s12 m6" style="margin-top: 0; margin-bottom: 0;">
                  <input id="kifu_title" type="text" class="nm-input" bind:value={title} placeholder="対局名など (省略時は自動設定)" style="margin-bottom: 0; border-radius: 0 !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important;" />
                  <label for="kifu_title" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text); font-weight: 600;">タイトル</label>
                </div>

                <div class="input-field col s12" style="margin-top: 14px; margin-bottom: 0;">
                  <textarea id="sgf_data" class="materialize-textarea nm-textarea nm-input" style="font-family: monospace; min-height: 120px; margin-bottom: 0; border-radius: 0 !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important;" bind:value={sgfData} placeholder="(;GM[1]FF[4]...)"></textarea>
                  <label for="sgf_data" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text); font-weight: 600;">SGFデータ (必須)</label>
                </div>
              </div>
            </div>
            <div class="card-action right-align" style="border-top: 1.5px solid var(--wc-text); padding: 16px 0 0 0; margin-top: 16px; display: flex; justify-content: flex-end; gap: 12px; background: transparent;">
              <button class="nm-btn-flat" style="font-weight: bold;" onclick={() => { showUploadForm = false; title = ""; sgfData = ""; }}>キャンセル</button>
              <button class="nm-btn-primary" style="border-radius: 0 !important; border: 1.5px solid var(--wc-text) !important; box-shadow: 3px 3px 0px var(--wc-text) !important; font-weight: bold;" disabled={!sgfData.trim() || uploading} onclick={handleUpload}>
                {#if uploading}
                  保存中...
                {:else}
                  <i class="material-icons" style="font-size: 1.1rem; vertical-align: middle; margin-right: 4px;">check</i>登録する
                {/if}
              </button>
            </div>
          </div>
        </div>
      </div>
    {/if}
    
    <!-- Title wrapper inside main column -->
    <div style="border-bottom: 2.5px solid var(--wc-text); padding-bottom: 12px; margin-bottom: 28px; display: flex; justify-content: space-between; align-items: flex-end; position: relative;">
      <span style="font-family: 'Cormorant Garamond', serif; font-size: 2.4rem; font-weight: 800; font-style: italic; color: var(--wc-text); line-height: 1; letter-spacing: -0.01em;">
        {#if publicMode}
          {ownerUsername ? `${ownerUsername}’s Archive` : 'Public Collection'}
        {:else}
          Personal Library
        {/if}
      </span>
      <span class="em-collage-tag-pastel" style="font-family: 'JetBrains Mono', sans-serif; font-size: 0.65rem; color: var(--wc-text); font-weight: bold; letter-spacing: 0.05em; border: 1.5px solid var(--wc-text); padding: 2px 8px; box-shadow: 2px 2px 0px var(--wc-text); background: var(--wc-surface-alt);">{filteredKifus.length} RECORDS FOUND</span>
    </div>
    
    <!-- Archive Column Grid (Magazine View Spread) -->
    {#if loading}
      <div class="center-align" style="margin-top: 5rem;">
        <div class="nm-spinner" style="width: 48px; height: 48px; margin: 0 auto; border-top-color: var(--wc-text);"></div>
        <p class="text-muted" style="margin-top: 16px; font-size: 0.88rem; font-family: 'JetBrains Mono', monospace;">LOADING ARCHIVES...</p>
      </div>
    {:else if error}
      <div class="card-panel red lighten-4 red-text text-darken-4" style="border: 2px solid var(--wc-text); border-radius: 0; box-shadow: 4px 4px 0px var(--wc-text);">
        <i class="material-icons left">error</i>
        エラーが発生しました: {error}
      </div>
    {:else if kifus.length === 0}
      <div class="center-align" style="padding: 4rem 2rem; border: 2px dashed var(--wc-text); background: var(--wc-surface-alt); box-shadow: 5px 5px 0px rgba(0,0,0,0.05);">
        <i class="material-icons" style="font-size: 3.6rem; color: var(--wc-text); opacity: 0.35; margin-bottom: 12px;">layers_clear</i>
        <h5 style="font-family: 'Shippori Mincho B1', serif; font-weight: 700; color: var(--wc-text); margin-bottom: 8px;">登録されている棋譜がありません</h5>
        {#if !publicMode}
          <p class="text-muted" style="font-size: 0.82rem; max-width: 320px; margin: 0 auto;">「自分で棋譜を作成」または「SGFファイルアップロード」から登録を行ってください。</p>
        {:else}
          <p class="text-muted" style="font-size: 0.82rem; max-width: 320px; margin: 0 auto;">このユーザーが一般公開している棋譜はまだありません。</p>
        {/if}
      </div>
    {:else if filteredKifus.length === 0}
      <div class="center-align" style="padding: 4rem 2rem; border: 2px dashed var(--wc-text); background: var(--wc-surface-alt);">
        <i class="material-icons" style="font-size: 3.6rem; color: var(--wc-text); opacity: 0.35; margin-bottom: 12px;">search_off</i>
        <h5 style="font-family: 'Shippori Mincho B1', serif; font-weight: 700; color: var(--wc-text); margin-bottom: 8px;">条件に一致する棋譜が見つかりません</h5>
        <p class="text-muted" style="font-size: 0.82rem;">検索キーワードや日付の範囲を変更してお試しください。</p>
      </div>
    {:else}
      <div class="kifu-tile-grid">
        {#each filteredKifus as k, i (k.id)}
          <div class="kifu-tile-card animate-pop-in stagger-{(i % 5) + 1}">
            <button
              type="button"
              class="kifu-tile-click-area"
              onclick={() => dispatch('selectKifu', k.id)}
              aria-label="{k.title}の詳細を表示"
            >
              <!-- OGP画像エリア (正方形) -->
              <div class="kifu-tile-image-container">
                {#if k.has_ogp}
                  <img
                    src={publicMode ? `/api/u/${userId}/kifus/${k.id}/og-image` : `/api/kifus/${k.id}/og-image`}
                    alt="OGP preview"
                    class="kifu-tile-image"
                    loading="lazy"
                  />
                {:else}
                  <div class="kifu-tile-placeholder-wrap">
                    <svg viewBox="0 0 100 100" class="kifu-tile-placeholder">
                      <line x1="20" y1="50" x2="80" y2="50" stroke="var(--wc-text)" stroke-width="0.7" opacity="0.15" />
                      <line x1="50" y1="20" x2="50" y2="80" stroke="var(--wc-text)" stroke-width="0.7" opacity="0.15" />
                      <circle cx="50" cy="50" r="2.5" fill="var(--wc-text)" opacity="0.3" />
                      <circle cx="44" cy="44" r="10" fill="var(--wc-text)" opacity="0.15" />
                      <circle cx="56" cy="56" r="10" fill="var(--wc-surface)" stroke="var(--wc-text)" stroke-width="1" opacity="0.8" />
                    </svg>
                  </div>
                {/if}

                <!-- 右上：対局結果バッジ -->
                {#if k.result}
                  <span class="kifu-tile-result-badge">{k.result}</span>
                {/if}
              </div>

              <!-- カードテキストコンテンツ -->
              <div class="kifu-tile-info">
                <!-- タイトル -->
                <h4 class="kifu-tile-title font-mincho" title={k.title}>
                  {k.title}
                </h4>

                <!-- 対局者情報 -->
                <div class="kifu-tile-players font-sans">
                  <!-- 黒 -->
                  <div class="player-row-inline">
                    <span class="stone-dot stone-black"></span>
                    <span class="player-name-txt">{k.black_player || 'Unknown'}</span>
                    {#if k.black_rank}
                      <span class="player-rank-badge">{k.black_rank}</span>
                    {/if}
                  </div>
                  <!-- 白 -->
                  <div class="player-row-inline">
                    <span class="stone-dot stone-white"></span>
                    <span class="player-name-txt">{k.white_player || 'Unknown'}</span>
                    {#if k.white_rank}
                      <span class="player-rank-badge">{k.white_rank}</span>
                    {/if}
                  </div>
                </div>

                <!-- フッターメタ -->
                <div class="kifu-tile-footer font-mono">
                  <span>{k.game_date || 'Unknown'}</span>
                </div>
              </div>
            </button>

            <!-- 削除ボタン（非公開モード＝オーナー自身のみ・カードのボタンクリックと干渉させないために分離して絶対配置） -->
            {#if !publicMode}
              <button
                class="kifu-tile-delete-btn"
                onclick={(e) => handleDelete(k.id, e)}
                title="削除"
                aria-label="この棋譜を削除"
              >
                <i class="material-icons" aria-hidden="true" style="font-size: 1.15rem;">delete_outline</i>
              </button>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  /* ---- Page Header ---- */
  .list-page-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-end;
    flex-wrap: wrap;
    gap: 12px;
  }

  .list-page-title-wrap {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .list-page-title {
    margin: 0 !important;
    font-size: 2rem !important;
    font-weight: 600 !important;
    letter-spacing: 0.02em;
    color: var(--wc-text);
    font-family: 'Shippori Mincho B1', 'Noto Serif JP', serif;
    line-height: 1.3 !important;
  }

  .list-page-subtitle {
    margin: 0;
    font-size: 0.85rem;
    font-family: 'DM Sans', 'Noto Sans JP', sans-serif;
    color: var(--wc-text-muted);
  }

  .list-page-actions {
    display: flex;
    gap: 10px;
    flex-wrap: wrap;
    align-items: center;
  }

  /* ---- Filter ---- */
  .filter-card {
    border-left: 3px solid var(--wc-accent-warm) !important;
  }

  .filter-header {
    display: flex;
    align-items: center;
    margin-bottom: 14px;
  }

  .filter-label {
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--wc-accent);
    display: inline-flex;
    align-items: center;
    gap: 5px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    font-family: 'DM Sans', sans-serif;
  }

  .clear-filter-btn {
    cursor: pointer;
    font-size: 0.82rem;
    font-weight: 600;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: var(--wc-accent);
    font-family: 'DM Sans', sans-serif;
  }

  .clear-filter-btn:hover {
    text-decoration: underline;
    text-underline-offset: 3px;
  }

  /* ---- Kifu Cards (Editorial) ---- */
  .kifu-card-title {
    font-weight: 600;
    font-size: 1.1rem;
    color: var(--wc-text);
    margin-bottom: 0.5rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: calc(100% - 35px);
    letter-spacing: 0.02em;
    font-family: 'Shippori Mincho B1', 'Noto Serif JP', serif;
  }

  /* ---- Players ---- */
  .players-info {
    margin: 0.6rem 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .player-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  /* Stone dots — washi clay style */
  .stone-dot {
    display: inline-block;
    width: 13px;
    height: 13px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .stone-black {
    background: radial-gradient(circle at 32% 32%, #555, var(--wc-go-black));
    border: 1px solid rgba(0,0,0,0.6);
    box-shadow: 1px 1px 3px rgba(0,0,0,0.3);
  }

  .stone-white {
    background: radial-gradient(circle at 32% 32%, #ffffff, var(--wc-go-white));
    border: 1px solid var(--wc-border);
    box-shadow: 1px 1px 2px rgba(0,0,0,0.1);
  }

  .player-name {
    font-family: 'DM Sans', 'Noto Sans JP', sans-serif;
    font-weight: 500;
    font-size: 0.88rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 140px;
    color: var(--wc-text);
  }

  /* ---- Meta ---- */
  .kifu-meta {
    display: flex;
    flex-direction: column;
    gap: 2px;
    font-size: 0.76rem;
    color: var(--wc-text-muted);
    margin-top: 10px;
    padding-top: 8px;
    border-top: 1px dashed var(--wc-border);
    font-family: 'DM Sans', sans-serif;
  }

  /* ---- Card Action Bar ---- */
  .card-action-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-top: 1px solid var(--wc-border);
    padding: 12px 20px;
  }

  .open-label {
    font-family: 'DM Sans', 'Noto Sans JP', sans-serif;
    font-weight: 600;
    font-size: 0.8rem;
    color: var(--wc-accent);
    display: inline-flex;
    align-items: center;
    gap: 5px;
    letter-spacing: 0.05em;
  }

  .delete-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 30px;
    height: 30px;
    border-radius: 50%;
    border: none;
    background: transparent;
    color: var(--wc-text-muted);
    cursor: pointer;
    transition: all 0.2s ease;
    padding: 0;
  }

  .delete-btn:hover {
    background: rgba(180, 60, 60, 0.1);
    color: #B03030;
    transform: scale(1.1);
  }

  /* ---- Result Badge ---- */
  .wc-result-badge {
    background: var(--wc-surface);
    border: 1px solid var(--wc-border);
    box-shadow: var(--nm-shadow-outset-sm);
    padding: 3px 10px;
    border-radius: var(--wc-radius-sm);
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--wc-accent);
    font-family: 'DM Sans', 'JetBrains Mono', sans-serif;
    letter-spacing: 0.04em;
  }

  /* Drawer styles */
  .em-vogue-drawer {
    width: 380px;
  }

  .em-newspaper-headline {
    font-size: 2.5rem;
  }

  /* Mobile responsive */
  @media only screen and (max-width: 600px) {
    :global(.card-content) {
      padding: 14px !important;
    }
    .list-page-title {
      font-size: 1.5rem !important;
    }
    .em-vogue-drawer {
      width: 100% !important;
      padding: 30px 16px !important;
    }
    .em-newspaper-headline {
      font-size: 1.6rem !important;
      margin-bottom: 12px !important;
    }
    
    /* Stack players and metadata vertically on mobile to prevent overflow */
    .em-vogue-editorial-item > .card-content > div:last-child {
      flex-direction: column !important;
      align-items: flex-start !important;
      gap: 12px !important;
    }
    .kifu-meta {
      text-align: left !important;
      border-top: 1px dashed var(--wc-border) !important;
      width: 100% !important;
      padding-top: 8px !important;
      margin-top: 4px !important;
    }

    /* Expand filter drawer ribbon toggle for easier tapping */
    .em-vogue-drawer-toggle {
      padding: 18px 10px !important;
      font-size: 0.8rem !important;
      top: 30% !important;
    }
  }

  .animate-fade-in {
    animation: fadeIn 0.3s ease-out;
  }

  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(8px); }
    to { opacity: 1; transform: translateY(0); }
  }
  /* ---- Grid Tile Layout ---- */
  .kifu-tile-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 24px;
    margin-bottom: 3.5rem;
  }

  .kifu-tile-card {
    position: relative;
    border: 1.5px solid var(--wc-text);
    background: var(--wc-surface);
    box-shadow: 4px 4px 0px var(--wc-text);
    transition: transform 0.25s cubic-bezier(0.25, 0.46, 0.45, 0.94), box-shadow 0.25s cubic-bezier(0.25, 0.46, 0.45, 0.94);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .kifu-tile-card:hover {
    transform: translate(-3px, -3px);
    box-shadow: 7px 7px 0px var(--wc-text);
  }

  .kifu-tile-click-area {
    display: flex;
    flex-direction: column;
    width: 100%;
    border: none;
    background: transparent;
    padding: 0;
    margin: 0;
    text-align: left;
    color: inherit;
    cursor: pointer;
    font-family: inherit;
  }

  /* OGP Image area - strictly 1:1 aspect ratio */
  .kifu-tile-image-container {
    position: relative;
    width: 100%;
    aspect-ratio: 1 / 1;
    overflow: hidden;
    background: var(--wc-surface-alt);
    border-bottom: 1.5px solid var(--wc-text);
  }

  .kifu-tile-image {
    width: 100%;
    height: 100%;
    object-fit: cover;
    transition: transform 0.4s ease;
  }

  .kifu-tile-card:hover .kifu-tile-image {
    transform: scale(1.03);
  }

  /* OGP Placeholder style */
  .kifu-tile-placeholder-wrap {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: radial-gradient(circle, var(--wc-surface-alt) 60%, var(--wc-bg) 100%);
    padding: 20%;
    box-sizing: border-box;
  }

  .kifu-tile-placeholder {
    width: 100%;
    height: 100%;
    opacity: 0.85;
  }

  /* Overlay Badge */
  .kifu-tile-result-badge {
    position: absolute;
    top: 12px;
    right: 12px;
    background: var(--wc-accent-warm);
    color: var(--wc-text);
    border: 1px solid var(--wc-text);
    box-shadow: 2px 2px 0px var(--wc-text);
    font-size: 0.65rem;
    font-weight: bold;
    padding: 3px 8px;
    font-family: 'JetBrains Mono', monospace;
    z-index: 5;
  }

  /* Card Texts */
  .kifu-tile-info {
    padding: 16px;
    display: flex;
    flex-direction: column;
    flex-grow: 1;
    width: 100%;
    box-sizing: border-box;
  }

  .kifu-tile-title {
    margin: 0 0 12px 0 !important;
    font-size: 1.15rem !important;
    font-weight: 900 !important;
    line-height: 1.25 !important;
    color: var(--wc-text);
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    height: 2.5rem;
    word-break: break-word;
  }

  .kifu-tile-players {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-bottom: 12px;
  }

  .player-row-inline {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .player-name-txt {
    font-weight: 700;
    font-size: 0.82rem;
    color: var(--wc-text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 130px;
  }

  .player-rank-badge {
    font-size: 0.62rem;
    font-weight: bold;
    font-family: 'JetBrains Mono', monospace;
    background: var(--wc-surface-alt);
    border: 1px solid var(--wc-text);
    padding: 1px 5px;
    color: var(--wc-text-muted);
  }

  .kifu-tile-footer {
    margin-top: auto;
    font-size: 0.68rem;
    color: var(--wc-text-muted);
    opacity: 0.8;
  }

  /* Absolute delete button - overlay on hover */
  .kifu-tile-delete-btn {
    position: absolute;
    bottom: 12px;
    right: 12px;
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--wc-surface);
    border: 1px solid var(--wc-text);
    color: var(--wc-text);
    cursor: pointer;
    opacity: 0;
    transform: translateY(5px);
    transition: opacity 0.2s ease, transform 0.2s ease, background 0.2s ease;
    box-shadow: 1.5px 1.5px 0 var(--wc-text);
    z-index: 10;
  }

  .kifu-tile-card:hover .kifu-tile-delete-btn {
    opacity: 0.8;
    transform: translateY(0);
  }

  .kifu-tile-delete-btn:hover {
    opacity: 1 !important;
    background: var(--wc-accent-warm) !important;
    color: var(--wc-text) !important;
  }

  /* Mobile responsiveness adjustments */
  @media only screen and (max-width: 600px) {
    .kifu-tile-grid {
      grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
      gap: 12px;
    }
    .kifu-tile-title {
      font-size: 0.95rem !important;
      height: 2.1rem;
    }
    .player-name-txt {
      max-width: 75px;
      font-size: 0.75rem;
    }
    .player-rank-badge {
      font-size: 0.58rem;
      padding: 0px 3px;
    }
    .kifu-tile-info {
      padding: 10px;
    }
    .kifu-tile-result-badge {
      font-size: 0.58rem;
      padding: 2px 5px;
      top: 6px;
      right: 6px;
    }
    /* Always show delete button on mobile (no hover state) */
    .kifu-tile-delete-btn {
      opacity: 0.75;
      transform: none;
      bottom: 8px;
      right: 8px;
      width: 24px;
      height: 24px;
    }
  }
</style>

