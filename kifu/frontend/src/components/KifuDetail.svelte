<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import Board from './Board.svelte';
  import ShareDialog from './ShareDialog.svelte';
  import { SgfPlayer, stringifySgf } from '../lib/sgfPlayer';
  import type { SgfNode } from '../lib/sgfPlayer';
  import { auth } from '../lib/auth.svelte';

  let {
    kifuId = "",
    shareToken = "",
    userId = "",
    onBack = () => {}
  } = $props<{
    kifuId?: string;
    shareToken?: string;
    userId?: string;
    onBack?: () => void;
  }>();

  let showShareDialog = $state(false);

  interface KifuDetailData {
    id: string;
    title: string;
    sgf_data: string;
    handicap: number;
    komi: number;
    black_player?: string;
    black_rank?: string;
    white_player?: string;
    white_rank?: string;
    result?: string;
    game_date?: string;
    share_token?: string;
    share_expires_at?: string;
    uploaded_by?: string;
    is_private?: boolean;
  }

  interface ReviewItem {
    id: string;
    move_number: number;
    reviewer_name: string;
    comment: string;
    variations?: string;
  }

  interface CommentItem {
    author: string;
    text: string;
  }

  interface BranchItem {
    label: string;
    node: SgfNode;
    originalIndex: number;
  }

  let kifu = $state<KifuDetailData | null>(null);
  let loading = $state(true);
  let error = $state<string | null>(null);

  // Player state
  let player = $state<SgfPlayer | null>(null);
  let boardState = $state<number[][]>([]);
  let lastMove = $state<{ x: number; y: number } | null>(null);
  let captives = $state({ B: 0, W: 0 });
  let currentIndex = $state(0);
  let maxIndex = $state(0);
  let comments = $state<CommentItem[]>([]); // Comments at current move
  let alternativeBranches = $state<BranchItem[]>([]); // Sibling nodes (alternative moves)

  // Autoplay state
  let autoplayInterval = $state<any>(null);
  let isAutoplay = $state(false);
  let autoplaySpeed = $state(2000); // ms

  // Review mode state
  let reviewMode = $state(false);
  let reviewerName = $state("");
  let reviewComment = $state("");
  let isAddingReview = $state(false);
  let reviewList = $state<ReviewItem[]>([]); // Review items fetched from database
  let isViewingVariation = $state(false);
  let activeReviewer = $state("");
  let isGameInfoExpanded = $state(false);

  // For board config
  let boardSize = $state(19);
  let currentTurn = $state(1); // 1: Black, 2: White (used for review mode placing stones)

  const isPublicProfileMode = $derived(!!userId && !!kifuId);
  const isOwner = $derived(!!kifu && auth.isLoggedIn && kifu.uploaded_by === auth.userId);

  // Type helper for Materialize global object
  const getM = () => (window as any).M;

  async function loadKifu() {
    loading = true;
    try {
      let kifuRes: Response;
      let reviewRes: Response;

      if (isPublicProfileMode) {
        kifuRes = await fetch(`/api/u/${userId}/kifus/${kifuId}`);
        if (!kifuRes.ok) throw new Error("Failed to fetch public kifu details");
        kifu = await kifuRes.json();

        reviewRes = await fetch(`/api/u/${userId}/kifus/${kifuId}/reviews`);
      } else if (shareToken) {
        kifuRes = await fetch(`/api/share/${shareToken}`);
        if (!kifuRes.ok) throw new Error("Failed to fetch shared kifu details");
        kifu = await kifuRes.json();

        reviewRes = await fetch(`/api/share/${shareToken}/reviews`);
      } else {
        kifuRes = await fetch(`/api/kifus/${kifuId}`, {
          headers: auth.getHeaders()
        });
        if (!kifuRes.ok) throw new Error("Failed to fetch kifu details");
        kifu = await kifuRes.json();

        reviewRes = await fetch(`/api/kifus/${kifuId}/reviews`, {
          headers: auth.getHeaders()
        });
      }

      if (!kifu) throw new Error("Kifu data is null");
      boardSize = kifu.handicap > 0 || kifu.sgf_data.includes("SZ[19]") ? 19 : 19;
      
      if (reviewRes.ok) {
        reviewList = await reviewRes.json();
      }

      // Initialize SgfPlayer
      player = new SgfPlayer(kifu.sgf_data, boardSize);
      
      // Merge review comments and variations into SGF tree
      mergeReviewsIntoPlayer();

      updatePlayerState();
    } catch (err: any) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  // Recursively marks SGF nodes as variation branch
  function markAsVariation(node: SgfNode, reviewerName: string) {
    (node as any).is_variation = true;
    (node as any).reviewer_name = reviewerName;
    for (const child of node.children) {
      markAsVariation(child, reviewerName);
    }
  }

  // Merges comments and variation subtrees from Database into the active SgfPlayer tree
  function mergeReviewsIntoPlayer() {
    if (!player || !reviewList || reviewList.length === 0) return;

    for (const rev of reviewList) {
      // Find the target node in SGF based on move_number
      let node: SgfNode | null = player.root;
      let count = 0;
      
      // Traverse to the targeted move_number
      while (node && count < rev.move_number) {
        if (node.children.length > 0) {
          node = node.children[0];
          count++;
        } else {
          break;
        }
      }

      if (node) {
        if (count === rev.move_number) {
          // Case 1: Target move is within the existing primary path
          // Add comment
          if (!node.properties["C"]) {
            node.properties["C"] = [];
          }
          node.properties["C"].push(`${rev.reviewer_name}: ${rev.comment}`);

          // Add variation if present
          if (rev.variations && rev.variations.trim() !== "") {
            try {
              // Variations is stored as an SGF node/tree
              const varPlayer = new SgfPlayer(rev.variations, boardSize);
              if (varPlayer.root && node.parent) {
                // Ensure it's not already added
                const targetProps = JSON.stringify(varPlayer.root.properties);
                const alreadyExists = node.parent.children.some(child => {
                  return JSON.stringify(child.properties) === targetProps;
                });
                if (!alreadyExists) {
                  markAsVariation(varPlayer.root, rev.reviewer_name);
                  varPlayer.root.parent = node.parent;
                  node.parent.children.push(varPlayer.root);
                }
              }
            } catch (e) {
              console.error("Failed to parse variation SGF:", e);
            }
          }
        } else if (count === rev.move_number - 1) {
          // Case 2: Target move is directly after the last move of the primary path
          // Attach the variation root as a child of the last node
          if (rev.variations && rev.variations.trim() !== "") {
            try {
              const varPlayer = new SgfPlayer(rev.variations, boardSize);
              if (varPlayer.root) {
                const targetProps = JSON.stringify(varPlayer.root.properties);
                const alreadyExists = node.children.some(child => {
                  return JSON.stringify(child.properties) === targetProps;
                });
                if (!alreadyExists) {
                  // Append comment directly onto the variation root node since no primary node exists at this index
                  if (rev.comment && rev.comment.trim() !== "") {
                    if (!varPlayer.root.properties["C"]) {
                      varPlayer.root.properties["C"] = [];
                    }
                    varPlayer.root.properties["C"].push(`${rev.reviewer_name}: ${rev.comment}`);
                  }
                  markAsVariation(varPlayer.root, rev.reviewer_name);
                  varPlayer.root.parent = node;
                  node.children.push(varPlayer.root);
                }
              }
            } catch (e) {
              console.error("Failed to parse variation SGF:", e);
            }
          } else {
            // No variation, just comment on the subsequent empty slot. Attach to the last node.
            if (!node.properties["C"]) {
              node.properties["C"] = [];
            }
            node.properties["C"].push(`${rev.reviewer_name}: ${rev.comment}`);
          }
        }
      }
    }
    
    // Recalculate main path after merge
    player.initGame();
  }

  function updatePlayerState() {
    if (!player) return;
    const state = player.getCurrentState();
    boardState = state.board;
    lastMove = state.lastMove;
    captives = state.captives;
    currentIndex = player.currentIndex;
    maxIndex = player.history.length - 1;

    // Get current comment from node
    comments = [];
    if (state.node && state.node.properties["C"]) {
      for (const rawComment of state.node.properties["C"]) {
        const colonIndex = rawComment.indexOf(":");
        if (colonIndex !== -1) {
          comments.push({
            author: rawComment.substring(0, colonIndex).trim(),
            text: rawComment.substring(colonIndex + 1).trim()
          });
        } else {
          comments.push({
            author: "コメント",
            text: rawComment
          });
        }
      }
    }

    // Determine current turn color for review mode placing stones
    // Default: alternate based on move number
    currentTurn = currentIndex % 2 === 0 ? 1 : 2; // Black on even index (0 is handicap/root, 1 is Black first move if no handicap)
    // Adjust if handicap exists
    if (kifu && kifu.handicap > 0) {
      currentTurn = currentIndex % 2 === 0 ? 2 : 1; // White on even index (0 is AB, 1 is White first move)
    }

    // Check if currently on a variation branch
    const currentNode = state.node;
    let variationFound = false;
    let reviewer = "";

    if (currentNode) {
      let temp: SgfNode | null = currentNode;
      while (temp) {
        if ((temp as any).is_variation) {
          variationFound = true;
          reviewer = (temp as any).reviewer_name || "";
          break;
        }
        temp = temp.parent;
      }
    }
    isViewingVariation = variationFound;
    activeReviewer = reviewer;

    // Get alternative branches (siblings)
    alternativeBranches = player.getAlternativeBranches().map((node) => {
      let moveLabel = "変化図";
      if (node.properties.B) moveLabel = `黒 ${node.properties.B[0]}`;
      else if (node.properties.W) moveLabel = `白 ${node.properties.W[0]}`;
      
      const nodeReviewer = (node as any).reviewer_name || "";
      const label = nodeReviewer ? `${nodeReviewer} さんの指導 (${moveLabel})` : `変化図 (${moveLabel})`;
      
      let originalIndex = -1;
      if (node.parent) {
        originalIndex = node.parent.children.indexOf(node);
      }
      
      return {
        label: label,
        node: node,
        originalIndex: originalIndex
      };
    });
  }

  // Navigation handlers
  function goFirst() {
    if (player && player.goTo(0)) updatePlayerState();
  }

  function goLast() {
    if (player && player.goTo(maxIndex)) updatePlayerState();
  }

  function stepBack(steps = 1) {
    if (!player) return;
    let target = Math.max(0, currentIndex - steps);
    if (player.goTo(target)) updatePlayerState();
  }

  // eslint-disable-next-line no-undef
  function stepForward(steps = 1) {
    if (!player) return;
    let target = Math.min(maxIndex, currentIndex + steps);
    if (player.goTo(target)) updatePlayerState();
  }

  function handleSliderChange(e: Event) {
    const target = e.target as HTMLInputElement;
    const idx = parseInt(target.value);
    if (player && player.goTo(idx)) updatePlayerState();
  }

  function selectBranch(branchIndex: number) {
    if (player && player.selectBranch(branchIndex)) {
      updatePlayerState();
    }
  }

  // Autoplay controls
  function toggleAutoplay() {
    if (isAutoplay) {
      stopAutoplay();
    } else {
      startAutoplay();
    }
  }

  function startAutoplay() {
    isAutoplay = true;
    autoplayInterval = setInterval(() => {
      if (currentIndex < maxIndex) {
        stepForward();
      } else {
        stopAutoplay();
      }
    }, autoplaySpeed);
  }

  function stopAutoplay() {
    isAutoplay = false;
    if (autoplayInterval) {
      clearInterval(autoplayInterval);
      autoplayInterval = null;
    }
  }

  // Handle board click (Review / Edit mode)
  async function handleIntersectionClick(e: CustomEvent<{ x: number; y: number }>) {
    const { x, y } = e.detail;

    if (!reviewMode) {
      const M = getM();
      if (M) {
        M.toast({ html: '盤面を編集するには「添削モード」を有効にしてください', classes: 'amber darken-2' });
      }
      return;
    }

    if (!player) return;

    // Add move to player
    const res = player.addMove(x, y, currentTurn);
    if (res.success) {
      if (res.isNew && res.node) {
        const reviewer = reviewerName.trim() || auth.username || "あなた";
        markAsVariation(res.node, reviewer);
      }
      updatePlayerState();
      
      // If it created a new branch/variation, notify the user
      if (res.isNew) {
        const M = getM();
        if (M) {
          M.toast({ html: '新しい変化図を作成しました', classes: 'green darken-2' });
        }
      }
    } else {
      const M = getM();
      if (M) {
        M.toast({ html: 'エラー: ' + res.error, classes: 'red' });
      }
    }
  }

  // Save Review Comment
  async function handleSaveReview() {
    if (!reviewerName.trim() || !reviewComment.trim()) {
      const M = getM();
      if (M) {
        M.toast({ html: '添削者名とコメントを入力してください', classes: 'amber' });
      }
      return;
    }

    if (!player) return;

    isAddingReview = true;
    try {
      const state = player.getCurrentState();
      
      // If we are on a variation branch, we want to extract the variation SGF sub-tree
      let variationsSgf = "";
      const currentNode = state.node;
      
      // If this node is not part of the main path, it's a variation.
      // Generate standard SGF representation for this variation branch.
      if (currentNode && currentNode.parent) {
        const siblings = currentNode.parent.children;
        // If there is more than 1 child, or the child is not the primary child (index 0)
        // we serialise the current branch.
        const isMainBranch = siblings[0] === currentNode;
        if (!isMainBranch) {
          // Serialize the subtree starting from this node
          // Basic serialization:
          variationsSgf = serializeSubtree(currentNode);
        }
      }

      let res: Response;
      if (isPublicProfileMode) {
        res = await fetch(`/api/u/${userId}/kifus/${kifuId}/reviews`, {
          method: 'POST',
          headers: auth.getHeaders(),
          body: JSON.stringify({
            move_number: currentIndex,
            node_path: String(currentIndex),
            reviewer_name: reviewerName.trim(),
            comment: reviewComment.trim(),
            variations: variationsSgf
          })
        });
      } else if (shareToken) {
        res = await fetch(`/api/share/${shareToken}/reviews`, {
          method: 'POST',
          headers: auth.getHeaders(),
          body: JSON.stringify({
            move_number: currentIndex,
            node_path: String(currentIndex),
            reviewer_name: reviewerName.trim(),
            comment: reviewComment.trim(),
            variations: variationsSgf
          })
        });
      } else {
        res = await fetch(`/api/kifus/${kifuId}/reviews`, {
          method: 'POST',
          headers: auth.getHeaders(),
          body: JSON.stringify({
            move_number: currentIndex,
            node_path: String(currentIndex),
            reviewer_name: reviewerName.trim(),
            comment: reviewComment.trim(),
            variations: variationsSgf
          })
        });
      }

      if (!res.ok) throw new Error("Failed to save review comment");

      const savedReview = await res.json();
      
      // Add local review list and merge
      reviewList = [...reviewList, savedReview];
      mergeReviewsIntoPlayer();
      updatePlayerState();

      // Reset comment text (keep reviewer name for convenience)
      reviewComment = "";
      
      const M = getM();
      if (M) {
        M.toast({ html: '添削コメントを保存しました！', classes: 'green darken-1' });
      }
    } catch (err: any) {
      const M = getM();
      if (M) {
        M.toast({ html: 'エラー: ' + err.message, classes: 'red' });
      }
    } finally {
      isAddingReview = false;
    }
  }

  // Simple serialization helper for a variation branch node
  function serializeSubtree(node: SgfNode) {
    return stringifySgf(node);
  }

  // Clean up timers
  onDestroy(() => {
    stopAutoplay();
  });

  onMount(() => {
    if (auth.username) {
      reviewerName = auth.username;
    }
    loadKifu();
  });
</script>

<div class="row" style="margin-top: 1rem;">
  <!-- Header Navigation -->
  <div class="col s12 d-flex align-center justify-between" style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 1rem; flex-wrap: wrap; gap: 10px;">
    <div style="display: flex; align-items: center;">
      <button class="btn-flat waves-effect brown-text" on:click={onBack} style="padding-left: 0;">
        <i class="material-icons left">arrow_back</i>戻る
      </button>
      {#if kifu}
        <h5 class="brown-text text-darken-4" style="margin: 0; font-weight: 500; margin-left: 1rem;">{kifu.title}</h5>
      {/if}
    </div>
    {#if kifu && isOwner}
      <button class="btn waves-effect waves-light brown lighten-1" on:click={() => showShareDialog = true}>
        <i class="material-icons left">share</i>共有設定
      </button>
    {/if}
  </div>

  {#if loading}
    <div class="col s12 center-align" style="margin-top: 5rem;">
      <div class="preloader-wrapper big active">
        <div class="spinner-layer spinner-brown-only">
          <div class="circle-clipper left"><div class="circle"></div></div>
          <div class="gap-patch"><div class="circle"></div></div>
          <div class="circle-clipper right"><div class="circle"></div></div>
        </div>
      </div>
      <p class="grey-text">棋譜を構築中...</p>
    </div>
  {:else if error}
    <div class="col s12">
      <div class="card-panel red lighten-4 red-text text-darken-4">
        <i class="material-icons left">error</i>
        エラーが発生しました: {error}
      </div>
    </div>
  {:else}
    <!-- Main UI Grid -->
    <!-- Left Column: Go Board & Controls -->
    <div class="col s12 l6 center-align">
      <div class="board-wrapper {isViewingVariation ? 'viewing-variation' : ''}" style="position: relative; display: inline-block;">
        {#if isViewingVariation}
          <div class="variation-badge animate-fade-in" style="position: absolute; top: 10px; left: 10px; z-index: 10; background: linear-gradient(135deg, #ff9800, #f57c00); color: #fff; padding: 6px 12px; border-radius: 20px; font-weight: bold; font-size: 0.85rem; display: flex; align-items: center; gap: 4px; box-shadow: 0 4px 10px rgba(0,0,0,0.2); border: 1px solid rgba(255,255,255,0.2);">
            <i class="material-icons" style="font-size: 1rem; vertical-align: middle;">call_split</i>
            <span>指導手順: {activeReviewer ? `${activeReviewer} さん` : 'あなた'}</span>
          </div>
        {/if}
        <Board
          board={boardState}
          size={boardSize}
          lastMove={lastMove}
          interactive={reviewMode}
          turnColor={currentTurn}
          on:intersectionClick={handleIntersectionClick}
        />
      </div>

      <!-- Playback Controls -->
      <div class="controls-panel card white" style="margin-top: 1rem; border-radius: 8px;">
        <div class="card-content" style="padding: 12px 20px;">
          <!-- Slider -->
          <div class="range-field d-flex align-center" style="display: flex; align-items: center; margin-bottom: 0.5rem;">
            <span style="font-weight: 500; min-width: 50px;" class="brown-text">{currentIndex} / {maxIndex}手</span>
            <input
              type="range"
              min="0"
              max={maxIndex}
              value={currentIndex}
              on:input={handleSliderChange}
              style="margin: 0 15px; flex-grow: 1;"
            />
          </div>

          <!-- Buttons Row -->
          <div class="buttons-row" style="display: flex; justify-content: center; gap: 8px; flex-wrap: wrap;">
            <button class="btn-flat btn-floating waves-effect brown-text" on:click={goFirst} title="最初へ">
              <i class="material-icons">first_page</i>
            </button>
            <button class="btn-flat btn-floating waves-effect brown-text" on:click={() => stepBack(10)} title="10手戻る">
              <i class="material-icons">fast_rewind</i>
            </button>
            <button class="btn-flat btn-floating waves-effect brown-text" on:click={() => stepBack(1)} title="1手戻る">
              <i class="material-icons">navigate_before</i>
            </button>
            <button class="btn btn-floating waves-effect waves-light brown" on:click={toggleAutoplay} title={isAutoplay ? '一時停止' : '自動再生'}>
              <i class="material-icons">{isAutoplay ? 'pause' : 'play_arrow'}</i>
            </button>
            <button class="btn-flat btn-floating waves-effect brown-text" on:click={() => stepForward(1)} title="1手進む">
              <i class="material-icons">navigate_next</i>
            </button>
            <button class="btn-flat btn-floating waves-effect brown-text" on:click={() => stepForward(10)} title="10手進む">
              <i class="material-icons">fast_forward</i>
            </button>
            <button class="btn-flat btn-floating waves-effect brown-text" on:click={goLast} title="最後へ">
              <i class="material-icons">last_page</i>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Right Column: Game Info, Comments & Variations -->
    <div class="col s12 l6 text-left" style="text-align: left;">
      <!-- Game Info Card (Collapsible) -->
      {#if kifu}
      <div class="card white game-info-card hoverable" style="border-radius: 8px; margin-top: 0; border: 1px solid #efebe9; transition: all 0.25s ease;">
        <!-- Card Header Toggle (Click to Toggle) -->
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div class="card-header-toggle cursor-pointer" on:click={() => isGameInfoExpanded = !isGameInfoExpanded} style="padding: 12px 20px; display: flex; align-items: center; justify-content: space-between; cursor: pointer; user-select: none;">
          <div style="display: flex; align-items: center; gap: 12px; flex-wrap: wrap;">
            <span class="brown-text text-darken-3" style="font-weight: 600; font-size: 1rem; display: flex; align-items: center; gap: 6px; margin-right: 4px;">
              <i class="material-icons" style="font-size: 1.15rem;">info_outline</i>
              対局情報
            </span>
            <span class="grey-text text-darken-3" style="font-size: 0.9rem; font-weight: 500; display: inline-flex; align-items: center; flex-wrap: wrap; gap: 6px;">
              <span class="font-weight-500">● {kifu.black_player || '不明'}</span>
              <span class="grey-text text-darken-1" style="font-size: 0.8rem;">(石 {captives.W})</span>
              <span class="grey-text" style="font-size: 0.75rem;">vs</span>
              <span class="font-weight-500">○ {kifu.white_player || '不明'}</span>
              <span class="grey-text text-darken-1" style="font-size: 0.8rem;">(石 {captives.B})</span>
              {#if kifu.result}
                <span class="brown-text lighten-1-text" style="margin-left: 6px; font-size: 0.8rem; padding: 1px 8px; background: #efebe9; border-radius: 12px; font-weight: 600;">{kifu.result}</span>
              {/if}
            </span>
          </div>
          <button class="btn-flat btn-floating waves-effect waves-circle" style="width: 32px; height: 32px; line-height: 32px; display: flex; align-items: center; justify-content: center; margin: 0; background: transparent;">
            <i class="material-icons" style="font-size: 1.4rem; transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1); transform: {isGameInfoExpanded ? 'rotate(180deg)' : 'rotate(0deg)'}; color: #5d4037;">keyboard_arrow_down</i>
          </button>
        </div>

        <!-- Expanded details -->
        {#if isGameInfoExpanded}
          <div class="card-content" style="padding: 0 20px 20px 20px; border-top: 1px dashed #efebe9; transition: all 0.3s ease;">
            <div class="row" style="margin-bottom: 0; margin-top: 12px;">
              <div class="col s6 font-weight-500">
                <span class="black-text" style="font-size: 0.9rem; font-weight: 600;">● 黒番:</span> 
                <span class="brown-text text-darken-2">{kifu.black_player || '不明'}</span> {kifu.black_rank ? `(${kifu.black_rank})` : ''}
                <div class="grey-text text-darken-1" style="font-size: 0.8rem; margin-top: 4px; display: flex; align-items: center; gap: 4px;">
                  <i class="material-icons" style="font-size: 0.9rem;">toll</i>
                  取得した白石: <strong style="color: #3e2723; font-size: 0.9rem;">{captives.W}</strong>
                </div>
              </div>
              <div class="col s6 font-weight-500">
                <span class="black-text" style="font-size: 0.9rem; font-weight: 600;">○ 白番:</span> 
                <span class="brown-text text-darken-2">{kifu.white_player || '不明'}</span> {kifu.white_rank ? `(${kifu.white_rank})` : ''}
                <div class="grey-text text-darken-1" style="font-size: 0.8rem; margin-top: 4px; display: flex; align-items: center; gap: 4px;">
                  <i class="material-icons" style="font-size: 0.9rem;">toll</i>
                  取得した黒石: <strong style="color: #3e2723; font-size: 0.9rem;">{captives.B}</strong>
                </div>
              </div>
              <div class="col s12" style="margin-top: 14px; border-top: 1px solid #efebe9; padding-top: 10px; font-size: 0.85rem; display: flex; flex-wrap: wrap; gap: 20px; color: #4e342e;">
                <span><strong>コミ:</strong> {kifu.komi}目</span>
                <span><strong>置き石:</strong> {kifu.handicap}子</span>
                <span><strong>結果:</strong> {kifu.result || 'なし'}</span>
                <span><strong>対局日:</strong> {kifu.game_date || '未登録'}</span>
              </div>
            </div>
          </div>
        {/if}
      </div>
      {/if}

      <!-- Branches & Variations Card -->
      {#if alternativeBranches.length > 0}
        <div class="card amber lighten-5 border-amber" style="border-radius: 8px;">
          <div class="card-content" style="padding: 12px 20px;">
            <span class="valign-wrapper amber-text text-darken-4" style="font-weight: 500;">
              <i class="material-icons left" style="font-size: 1.2rem;">call_split</i> 変化図があります
            </span>
            <div style="display: flex; gap: 8px; flex-wrap: wrap; margin-top: 8px;">
              <!-- Main branch return button if not on primary branch -->
              <!-- Display other branch buttons -->
              {#each alternativeBranches as branch}
                <button class="btn-small waves-effect waves-light brown lighten-1" on:click={() => selectBranch(branch.originalIndex)}>
                  {branch.label} に切り替え
                </button>
              {/each}
            </div>
          </div>
        </div>
      {/if}

      <!-- Comments / Review Card -->
      <div class="card white" style="border-radius: 8px; min-height: 180px; display: flex; flex-direction: column;">
        <div class="card-content" style="padding: 16px 20px; flex-grow: 1;">
          <span class="card-title brown-text text-darken-3" style="font-size: 1.15rem; font-weight: 500; margin-bottom: 8px;">
            検討・指導コメント (第 {currentIndex} 手)
          </span>

          {#if comments.length === 0}
            <div class="center-align grey-text text-lighten-1" style="padding: 2.5rem 0;">
              <i class="material-icons" style="font-size: 3rem;">chat_bubble_outline</i>
              <p style="margin: 5px 0 0 0; font-size: 0.9rem;">この手に対するコメントはありません</p>
            </div>
          {:else}
            <div class="comments-list" style="max-height: 250px; overflow-y: auto;">
              {#each comments as comment}
                <div class="comment-item" style="border-bottom: 1px solid #f0f0f0; padding: 8px 0;">
                  <span class="chip brown lighten-4 brown-text text-darken-4" style="font-weight: 500; height: 24px; line-height: 24px; margin-bottom: 4px;">
                    {comment.author}
                  </span>
                  <p style="margin: 0; padding-left: 4px; font-size: 0.95rem; white-space: pre-wrap;" class="grey-text text-darken-3">{comment.text}</p>
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <!-- Mode Toggle & Edit Panel Footer -->
        <div class="card-action grey lighten-4" style="border-radius: 0 0 8px 8px; padding: 10px 20px;">
          <div class="switch" style="margin-bottom: 10px;">
            <label class="black-text" style="font-weight: 500;">
              通常再生
              <input type="checkbox" bind:checked={reviewMode}>
              <span class="lever brown lighten-3"></span>
              添削モード (盤面入力可)
            </label>
          </div>

          {#if reviewMode}
            <div class="review-edit-panel animate-fade-in" style="margin-top: 10px; border-top: 1px dashed #ccc; padding-top: 12px;">
              <p class="grey-text text-darken-2" style="font-size: 0.85rem; margin: 0 0 10px 0;">
                <i class="material-icons left" style="font-size: 1rem; vertical-align: middle;">info</i>
                碁盤をクリックして石を置くと変化図を作成できます。この局面に対する指導コメントを入力してください。
              </p>
              
              <div class="row" style="margin-bottom: 0;">
                <div class="input-field col s12 m4" style="margin-top: 0; margin-bottom: 10px;">
                  <input id="reviewer_name" type="text" bind:value={reviewerName} placeholder="指導者名" style="margin-bottom: 0; height: 2.5rem;" />
                  <label for="reviewer_name" class="active">添削者</label>
                </div>
                <div class="input-field col s12 m8" style="margin-top: 0; margin-bottom: 10px;">
                  <input id="review_comment" type="text" bind:value={reviewComment} placeholder="指導・変化図の解説を入力してください" style="margin-bottom: 0; height: 2.5rem;" />
                  <label for="review_comment" class="active">コメント</label>
                </div>
                <div class="col s12 right-align">
                  <button class="btn waves-effect waves-light brown" disabled={!reviewerName || !reviewComment || isAddingReview} on:click={handleSaveReview}>
                    <i class="material-icons left">save</i>添削を保存
                  </button>
                </div>
              </div>
            </div>
          {/if}
        </div>
      </div>
    </div>
  {/if}
</div>

{#if showShareDialog && kifu}
  <ShareDialog 
    kifu={kifu} 
    onClose={() => showShareDialog = false} 
    onUpdate={(updatedKifu) => {
      if (kifu) {
        kifu = { ...kifu, ...updatedKifu } as KifuDetailData;
      }
    }} 
  />
{/if}

<style>
  .border-amber {
    border: 1px solid #ffe082;
  }
  .animate-fade-in {
    animation: fadeIn 0.25s ease-out;
  }
  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(5px); }
    to { opacity: 1; transform: translateY(0); }
  }
  .controls-panel {
    border-radius: 8px;
    overflow: hidden;
  }
  .comments-list::-webkit-scrollbar {
    width: 6px;
  }
  .comments-list::-webkit-scrollbar-track {
    background: #f1f1f1;
  }
  .comments-list::-webkit-scrollbar-thumb {
    background: #c1c1c1;
    border-radius: 3px;
  }
  .comments-list {
    scrollbar-width: thin;
  }

  /* Playback controls buttons styling */
  .buttons-row button {
    background-color: #efebe9 !important;
    color: #4e342e !important;
    transition: all 0.2s cubic-bezier(0.25, 0.8, 0.25, 1) !important;
    border: 1px solid #d7ccc8 !important;
    box-shadow: 0 1px 3px rgba(0,0,0,0.06) !important;
    display: inline-flex !important;
    align-items: center;
    justify-content: center;
  }

  /* Hover micro-animations */
  .buttons-row button:hover {
    background-color: #d7ccc8 !important;
    transform: scale(1.15) translateY(-2px) !important;
    box-shadow: 0 4px 8px rgba(0, 0, 0, 0.12) !important;
  }

  .buttons-row button:active {
    transform: scale(0.95) translateY(0) !important;
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.08) !important;
  }

  /* Emphasize the main play/pause button */
  .buttons-row button.btn.brown {
    background-color: #4e342e !important;
    color: #fff !important;
    border: none !important;
    box-shadow: 0 2px 4px rgba(0,0,0,0.12) !important;
  }

  .buttons-row button.btn.brown:hover {
    background-color: #5d4037 !important;
    transform: scale(1.18) translateY(-2px) !important;
    box-shadow: 0 5px 12px rgba(0,0,0,0.18) !important;
  }

  /* Mobile responsive adjustments */
  @media only screen and (max-width: 600px) {
    .buttons-row button {
      width: 36px !important;
      height: 36px !important;
      line-height: 36px !important;
    }
    .buttons-row button i {
      font-size: 1.3rem !important;
      line-height: 36px !important;
    }
    :global(.card-content) {
      padding: 12px !important;
    }
    :global(.card-action) {
      padding: 12px !important;
    }
    .review-edit-panel .row .input-field {
      margin-bottom: 5px !important;
    }
    h5 {
      font-size: 1.25rem !important;
    }
  }

  /* Board wrapper variation border and shading */
  .board-wrapper {
    border: 4px solid transparent;
    border-radius: 12px;
    padding: 4px;
    transition: all 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);
    display: inline-block;
  }

  .board-wrapper.viewing-variation {
    border-color: #ffb74d !important;
    background-color: #ffe0b2;
    box-shadow: 0 10px 30px rgba(255, 152, 0, 0.18) !important;
  }
</style>
