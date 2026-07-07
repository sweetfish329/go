<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { Toaster } from 'svelte-sonner';
  import KifuList from './components/KifuList.svelte';
  import KifuDetail from './components/KifuDetail.svelte';
  import Auth from './components/Auth.svelte';
  import UsernameDialog from './components/UsernameDialog.svelte';
  import KifuCreator from './components/KifuCreator.svelte';
  import AdminAuth from './components/AdminAuth.svelte';
  import AdminDashboard from './components/AdminDashboard.svelte';
  import { auth } from './lib/auth.svelte';

  let currentView = $state<"list" | "detail" | "auth" | "create" | "admin_auth" | "admin_dashboard" | "public_list">("list");
  let selectedKifuId = $state("");
  let selectedShareToken = $state("");
  let selectedUserId = $state("");
  let showUsernameDialog = $state(false);

  let siteSettings = $state({
    title: 'kifu_store',
    tab_name: 'kifu_store',
    favicon: '',
    theme_color: '#4e342e'
  });

  // Dark mode state: "light" | "dark" | "system"
  let themeMode = $state<"light" | "dark" | "system">("system");
  let mediaQuery: MediaQueryList;

  // Apply theme to document element
  function applyTheme(mode: "light" | "dark" | "system") {
    if (typeof window === 'undefined') return;
    const root = document.documentElement;
    if (mode === "light") {
      root.classList.add("light");
      root.classList.remove("dark");
    } else if (mode === "dark") {
      root.classList.add("dark");
      root.classList.remove("light");
    } else {
      root.classList.remove("light");
      root.classList.remove("dark");
    }
  }

  // Toggle dark/light/system theme
  function toggleTheme() {
    if (themeMode === "light") {
      themeMode = "dark";
    } else if (themeMode === "dark") {
      themeMode = "system";
    } else {
      themeMode = "light";
    }
    localStorage.setItem("theme", themeMode);
    applyTheme(themeMode);
  }

  // System theme change handler
  function handleSystemThemeChange() {
    if (themeMode === "system") {
      applyTheme("system");
    }
  }

  // Perform routing based on URL path
  function handleRouting() {
    const path = window.location.pathname;
    const params = new URLSearchParams(window.location.search);
    const admin = params.has('admin');

    // Handle OAuth2 callback redirect parameters
    const oauthToken = params.get('oauth_token');
    const oauthUsername = params.get('oauth_username');
    const oauthId = params.get('oauth_id');

    if (oauthToken && oauthUsername && oauthId) {
      auth.setLogin(oauthToken, oauthUsername, oauthId);
      // Clean query parameters from URL without reloading
      const url = new URL(window.location.href);
      url.searchParams.delete('oauth_token');
      url.searchParams.delete('oauth_username');
      url.searchParams.delete('oauth_id');
      window.history.replaceState({}, '', url.pathname + url.search);
      // ログイン成功 → 即座にリスト表示（以降のルーティング判定をスキップ）
      currentView = "list";
      return;
    }

    // Pattern: /u/:userId/:kifuId
    const kifuDetailMatch = path.match(/^\/u\/([^/]+)\/([^/]+)\/?$/);
    // Pattern: /u/:userId
    const publicListMatch = path.match(/^\/u\/([^/]+)\/?$/);

    if (kifuDetailMatch) {
      selectedUserId = kifuDetailMatch[1];
      selectedKifuId = kifuDetailMatch[2];
      selectedShareToken = "";
      currentView = "detail";
    } else if (publicListMatch) {
      selectedUserId = publicListMatch[1];
      selectedKifuId = "";
      selectedShareToken = "";
      currentView = "public_list";
    } else if (admin) {
      const adminToken = localStorage.getItem("admin_token");
      if (adminToken) {
        currentView = "admin_dashboard";
      } else {
        currentView = "admin_auth";
      }
    } else {
      // For backwards compatibility, still support ?share=TOKEN
      const share = params.get('share');
      if (share) {
        selectedShareToken = share;
        selectedUserId = "";
        selectedKifuId = "";
        currentView = "detail";
      } else {
        selectedUserId = "";
        selectedKifuId = "";
        selectedShareToken = "";
        if (!auth.isLoggedIn) {
          currentView = "auth";
        } else {
          currentView = "list";
        }
      }
    }
  }

  // Determine view on mount based on URL query params & auth state
  onMount(async () => {
    // Load theme from localStorage
    const savedTheme = localStorage.getItem("theme");
    if (savedTheme === "light" || savedTheme === "dark" || savedTheme === "system") {
      themeMode = savedTheme;
    }
    applyTheme(themeMode);

    // Setup OS theme change listener
    if (typeof window !== 'undefined') {
      mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
      mediaQuery.addEventListener('change', handleSystemThemeChange);
    }

    // Fetch site settings
    try {
      const res = await fetch('/api/site-settings');
      if (res.ok) {
        const data = await res.json();
        siteSettings.title = data.title || 'kifu_store';
        siteSettings.tab_name = data.tab_name || 'kifu_store';
        siteSettings.favicon = data.favicon || '';
        siteSettings.theme_color = data.theme_color || '#4e342e';
      }
    } catch (err) {
      console.error("Failed to load site settings:", err);
    }

    handleRouting();
    window.addEventListener('popstate', handleRouting);
  });

  onDestroy(() => {
    if (typeof window !== 'undefined') {
      window.removeEventListener('popstate', handleRouting);
      if (mediaQuery) {
        mediaQuery.removeEventListener('change', handleSystemThemeChange);
      }
    }
  });

  // Reactively apply settings to DOM
  $effect(() => {
    document.documentElement.style.setProperty('--theme-color', siteSettings.theme_color);
    document.title = siteSettings.tab_name;
    
    let faviconLink = document.querySelector("link[rel~='icon']") as HTMLLinkElement;
    if (!faviconLink) {
      faviconLink = document.createElement('link');
      faviconLink.rel = 'icon';
      document.head.appendChild(faviconLink);
    }
    if (siteSettings.favicon) {
      faviconLink.href = siteSettings.favicon;
    } else {
      faviconLink.href = '/vite.svg';
    }
  });

  function handleSelectKifu(event: CustomEvent<string>) {
    selectedKifuId = event.detail;
    if (selectedUserId) {
      window.history.pushState({}, '', `/u/${selectedUserId}/${selectedKifuId}`);
      handleRouting();
    } else {
      currentView = "detail";
    }
  }

  function handleBackToList() {
    if (selectedUserId && currentView === "detail") {
      window.history.pushState({}, '', `/u/${selectedUserId}`);
      handleRouting();
      return;
    }

    if (selectedShareToken) {
      window.history.replaceState({}, '', window.location.pathname);
      selectedShareToken = "";
    }
    
    selectedKifuId = "";
    selectedUserId = "";
    if (auth.isLoggedIn) {
      currentView = "list";
    } else {
      currentView = "auth";
    }
  }

  function handleGoHome() {
    window.history.pushState({}, '', '/');
    selectedUserId = "";
    selectedKifuId = "";
    selectedShareToken = "";
    handleRouting();
  }

  function handleLoginSuccess() {
    currentView = "list";
  }

  function handleLogout() {
    auth.logout();
    currentView = "auth";
  }
</script>

<!-- svelte-sonner Toast Container -->
<Toaster
  richColors
  position="top-center"
  toastOptions={{
    style: 'font-family: Outfit, sans-serif; border-radius: 16px; backdrop-filter: blur(12px); border: 1px solid rgba(255,255,255,0.5);'
  }}
/>

<div class="app-shell">
  <!-- Navigation Header — Y2K Holographic Glass Nav -->
  <nav class="nm-nav">
    <div class="nav-wrapper container">
      <!-- svelte-ignore a11y-missing-attribute -->
      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <!-- svelte-ignore a11y-no-static-element-interactions -->
      <a class="brand-logo cursor-pointer" onclick={handleGoHome} style="display: flex; align-items: center; gap: 10px; cursor: pointer;">
        <!-- 碁石モチーフ ロゴマーク -->
        <span class="nav-logo-icon" aria-hidden="true">◉</span>
        <span class="nav-logo-text font-cormorant">{siteSettings.title}</span>
      </a>

      <ul id="nav-mobile" class="right" style="display: flex; align-items: center; gap: 4px;">
        <!-- svelte-ignore a11y-missing-attribute -->
        <li><a onclick={handleGoHome} class="cursor-pointer nav-link">ホーム</a></li>

        {#if auth.isLoggedIn}
          <!-- svelte-ignore a11y-missing-attribute -->
          <li>
            <a
              onclick={() => showUsernameDialog = true}
              class="cursor-pointer nav-link nav-user-chip"
              title="ニックネームを変更"
            >
              <span class="user-avatar" aria-hidden="true">◎</span>
              <span>{auth.username}</span>
              <i class="material-icons" style="font-size: 0.9rem; opacity: 0.6;">edit</i>
            </a>
          </li>
          <!-- svelte-ignore a11y-missing-attribute -->
          <li>
            <a onclick={handleLogout} class="cursor-pointer nav-link" title="ログアウト">
              <i class="material-icons" style="font-size: 1.1rem;">logout</i>
            </a>
          </li>
        {/if}

        <!-- Theme Mode Toggle -->
        <li>
          <!-- svelte-ignore a11y-missing-attribute -->
          <a
            onclick={toggleTheme}
            class="cursor-pointer nav-theme-btn"
            title="テーマ切り替え ({themeMode === 'light' ? 'ライト固定' : themeMode === 'dark' ? 'ダーク固定' : 'システム連動'})"
          >
            <i class="material-icons" style="font-size: 1.2rem;">
              {themeMode === 'light' ? 'wb_sunny' : themeMode === 'dark' ? 'brightness_2' : 'brightness_auto'}
            </i>
          </a>
        </li>
      </ul>
    </div>
  </nav>

  <!-- Main Container -->
  <main class="container" style="padding-bottom: 5rem; padding-top: 2rem;">
    {#if currentView === "auth"}
      <Auth />
    {:else if currentView === "list"}
      <KifuList on:selectKifu={handleSelectKifu} on:createKifu={() => currentView = "create"} />
    {:else if currentView === "public_list"}
      <KifuList userId={selectedUserId} on:selectKifu={handleSelectKifu} />
    {:else if currentView === "create"}
      <KifuCreator onSaveSuccess={handleLoginSuccess} onCancel={handleBackToList} />
    {:else if currentView === "detail" && (selectedKifuId || selectedShareToken)}
      <KifuDetail kifuId={selectedKifuId} shareToken={selectedShareToken} userId={selectedUserId} onBack={handleBackToList} />
    {:else if currentView === "admin_auth"}
      <AdminAuth onLoginSuccess={() => currentView = "admin_dashboard"} />
    {:else if currentView === "admin_dashboard"}
      <AdminDashboard onLogout={() => currentView = "admin_auth"} />
    {/if}
  </main>
</div>

{#if showUsernameDialog}
  <UsernameDialog 
    onClose={() => showUsernameDialog = false} 
    onSuccess={() => {
      showUsernameDialog = false;
    }} 
  />
{/if}

<style>
  .app-shell {
    min-height: 100vh;
    position: relative;
  }

  main {
    min-height: 80vh;
    position: relative;
    z-index: 1;
  }

  .cursor-pointer {
    cursor: pointer !important;
  }

  /* ---- Nav Logo ---- */
  .nav-logo-icon {
    font-size: 1.3rem;
    color: var(--wc-accent);
    display: inline-block;
    line-height: 1;
    transition: var(--wc-transition-fast);
    opacity: 0.75;
  }

  .brand-logo:hover .nav-logo-icon {
    opacity: 1;
    transform: scale(1.1);
  }

  .nav-logo-text {
    font-family: 'Cormorant Garamond', 'Shippori Mincho B1', serif;
    font-size: 1.45rem;
    font-weight: 600;
    letter-spacing: 0.04em;
    color: var(--wc-text);
    font-style: italic;
  }

  /* ---- Nav Links ---- */
  .nav-link {
    display: inline-flex !important;
    align-items: center;
    gap: 5px;
    height: 40px !important;
    line-height: 40px !important;
    border-radius: 999px !important;
    padding: 0 14px !important;
    font-family: 'Outfit', sans-serif;
    font-weight: 500;
    font-size: 0.93rem;
    transition: all 0.2s ease;
    color: var(--nm-text-main) !important;
  }

  .nav-link:hover {
    background: var(--wc-accent-soft) !important;
    color: var(--wc-accent) !important;
  }

  /* User chip */
  .nav-user-chip {
    border: 1px solid var(--wc-border) !important;
    background: rgba(245, 240, 232, 0.45) !important;
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
  }

  .nav-user-chip:hover {
    background: var(--wc-accent-soft) !important;
    border-color: rgba(124, 107, 82, 0.4) !important;
  }

  .user-avatar {
    font-size: 0.95rem;
    color: var(--wc-accent);
    opacity: 0.7;
  }

  /* Theme toggle button */
  .nav-theme-btn {
    display: inline-flex !important;
    align-items: center;
    justify-content: center;
    width: 40px !important;
    height: 40px !important;
    border-radius: 50% !important;
    padding: 0 !important;
    color: var(--wc-accent) !important;
    border: 1px solid var(--wc-border) !important;
    background: rgba(245, 240, 232, 0.45) !important;
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    transition: all 0.25s ease;
  }

  .nav-theme-btn:hover {
    background: var(--wc-accent-soft) !important;
    border-color: rgba(124, 107, 82, 0.4) !important;
    transform: rotate(15deg);
  }
</style>
