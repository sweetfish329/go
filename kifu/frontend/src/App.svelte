<script lang="ts">
  import KifuList from './components/KifuList.svelte';
  import KifuDetail from './components/KifuDetail.svelte';

  let currentView: "list" | "detail" = "list";
  let selectedKifuId: string = "";

  function handleSelectKifu(event: CustomEvent<string>) {
    selectedKifuId = event.detail;
    currentView = "detail";
  }

  function handleBackToList() {
    currentView = "list";
    selectedKifuId = "";
  }
</script>

<div>
  <!-- Navigation Header -->
  <nav class="brown darken-3 z-depth-1">
    <div class="nav-wrapper container">
      <!-- svelte-ignore a11y-missing-attribute -->
      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <!-- svelte-ignore a11y-no-static-element-interactions -->
      <a class="brand-logo d-flex align-center cursor-pointer" on:click={handleBackToList} style="display: flex; align-items: center; gap: 8px; cursor: pointer; font-size: 1.4rem;">
        <i class="material-icons">grid_on</i>
        <span>囲碁 棋譜ストア & 添削</span>
      </a>
      <ul id="nav-mobile" class="right hide-on-med-and-down">
        <!-- svelte-ignore a11y-missing-attribute -->
        <li><a on:click={handleBackToList} class="cursor-pointer">ホーム</a></li>
        <li><a href="https://github.com/sweetfish329/go" target="_blank" rel="noreferrer">GitHub</a></li>
      </ul>
    </div>
  </nav>

  <!-- Main Container -->
  <main class="container" style="padding-bottom: 4rem;">
    {#if currentView === "list"}
      <KifuList on:selectKifu={handleSelectKifu} />
    {:else if currentView === "detail" && selectedKifuId}
      <KifuDetail kifuId={selectedKifuId} onBack={handleBackToList} />
    {/if}
  </main>
</div>

<style>
  main {
    min-height: 80vh;
  }
  .cursor-pointer {
    cursor: pointer !important;
  }
</style>
