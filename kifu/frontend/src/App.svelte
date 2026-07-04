<script lang="ts">
  import { onMount } from 'svelte';
  import KifuList from './components/KifuList.svelte';
  import KifuDetail from './components/KifuDetail.svelte';
  import Auth from './components/Auth.svelte';
  import UsernameDialog from './components/UsernameDialog.svelte';
  import { auth } from './lib/auth.svelte';

  let currentView = $state<"list" | "detail" | "auth">("list");
  let selectedKifuId = $state("");
  let selectedShareToken = $state("");
  let showUsernameDialog = $state(false);

  // Determine view on mount based on URL query params & auth state
  onMount(() => {
    const params = new URLSearchParams(window.location.search);
    const share = params.get('share');
    
    if (share) {
      selectedShareToken = share;
      currentView = "detail";
    } else {
      if (!auth.isLoggedIn) {
        currentView = "auth";
      } else {
        currentView = "list";
      }
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
  <nav class="brown darken-3 z-depth-1">
    <div class="nav-wrapper container">
      <!-- svelte-ignore a11y-missing-attribute -->
      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <!-- svelte-ignore a11y-no-static-element-interactions -->
      <a class="brand-logo d-flex align-center cursor-pointer" onclick={handleBackToList} style="display: flex; align-items: center; gap: 8px; cursor: pointer; font-size: 1.4rem;">
        <i class="material-icons">grid_on</i>
        <span>囲碁 棋譜ストア & 添削</span>
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
      <KifuList on:selectKifu={handleSelectKifu} />
    {:else if currentView === "detail" && (selectedKifuId || selectedShareToken)}
      <KifuDetail kifuId={selectedKifuId} shareToken={selectedShareToken} onBack={handleBackToList} />
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
