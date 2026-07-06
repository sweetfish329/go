<script lang="ts">
  import { onMount } from 'svelte';
  import KifuList from './components/KifuList.svelte';
  import KifuDetail from './components/KifuDetail.svelte';
  import Auth from './components/Auth.svelte';
  import UsernameDialog from './components/UsernameDialog.svelte';
  import KifuCreator from './components/KifuCreator.svelte';
  import AdminAuth from './components/AdminAuth.svelte';
  import AdminDashboard from './components/AdminDashboard.svelte';
  import { auth } from './lib/auth.svelte';

  let currentView = $state<"list" | "detail" | "auth" | "create" | "admin_auth" | "admin_dashboard">("list");
  let selectedKifuId = $state("");
  let selectedShareToken = $state("");
  let showUsernameDialog = $state(false);

  let siteSettings = $state({
    title: 'kifu_store',
    tab_name: 'kifu_store',
    favicon: '',
    theme_color: '#4e342e'
  });

  // Determine view on mount based on URL query params & auth state
  onMount(async () => {
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

    const params = new URLSearchParams(window.location.search);
    const share = params.get('share');
    const admin = params.has('admin');
    
    if (share) {
      selectedShareToken = share;
      currentView = "detail";
    } else if (admin) {
      const adminToken = localStorage.getItem("admin_token");
      if (adminToken) {
        currentView = "admin_dashboard";
      } else {
        currentView = "admin_auth";
      }
    } else {
      if (!auth.isLoggedIn) {
        currentView = "auth";
      } else {
        currentView = "list";
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
    currentView = "detail";
  }

  function handleBackToList() {
    if (selectedShareToken) {
      window.history.replaceState({}, '', window.location.pathname);
      selectedShareToken = "";
    }
    
    selectedKifuId = "";
    if (auth.isLoggedIn) {
      currentView = "list";
    } else {
      currentView = "auth";
    }
  }

  function handleLoginSuccess() {
    currentView = "list";
  }

  function handleLogout() {
    auth.logout();
    currentView = "auth";
  }
</script>

<div>
  <!-- Navigation Header -->
  <nav class="z-depth-1" style="background-color: var(--theme-color, #4e342e);">
    <div class="nav-wrapper container">
      <!-- svelte-ignore a11y-missing-attribute -->
      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <!-- svelte-ignore a11y-no-static-element-interactions -->
      <a class="brand-logo d-flex align-center cursor-pointer" onclick={handleBackToList} style="display: flex; align-items: center; gap: 8px; cursor: pointer; font-size: 1.4rem;">
        <i class="material-icons">grid_on</i>
        <span>{siteSettings.title}</span>
      </a>
      <ul id="nav-mobile" class="right">
        <!-- svelte-ignore a11y-missing-attribute -->
        <li><a onclick={handleBackToList} class="cursor-pointer">ホーム</a></li>
        {#if auth.isLoggedIn}
          <!-- svelte-ignore a11y-missing-attribute -->
          <li>
            <a onclick={() => showUsernameDialog = true} class="cursor-pointer" style="display: flex; align-items: center; gap: 4px; color: #efebe9; font-weight: 500; font-size: 0.95rem;">
              <i class="material-icons tiny" style="font-size: 1rem; margin-right: 4px;">edit</i>
              <span>{auth.username} さん</span>
            </a>
          </li>
          <!-- svelte-ignore a11y-missing-attribute -->
          <li><a onclick={handleLogout} class="cursor-pointer"><i class="material-icons left">exit_to_app</i>ログアウト</a></li>
        {/if}
      </ul>
    </div>
  </nav>

  <!-- Main Container -->
  <main class="container" style="padding-bottom: 4rem;">
    {#if currentView === "auth"}
      <Auth onLoginSuccess={handleLoginSuccess} />
    {:else if currentView === "list"}
      <KifuList on:selectKifu={handleSelectKifu} on:createKifu={() => currentView = "create"} />
    {:else if currentView === "create"}
      <KifuCreator onSaveSuccess={handleLoginSuccess} onCancel={handleBackToList} />
    {:else if currentView === "detail" && (selectedKifuId || selectedShareToken)}
      <KifuDetail kifuId={selectedKifuId} shareToken={selectedShareToken} onBack={handleBackToList} />
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
  main {
    min-height: 80vh;
  }
  .cursor-pointer {
    cursor: pointer !important;
  }
</style>
