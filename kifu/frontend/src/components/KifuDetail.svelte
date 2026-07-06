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
    reviewId?: string;
    reviewerName?: string;
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
  let editingReviewId = $state<string | null>(null);

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
      // Find the parent node in SGF based on move_number
      let parentNode: SgfNode | null = player.root;
      let count = 0;
      
      // Traverse to the targeted move_number
      while (parentNode && count < rev.move_number) {
        if (parentNode.children.length > 0) {
          parentNode = parentNode.children[0];
          count++;
        } else {
          break;
        }
      }

      if (parentNode && count === rev.move_number) {
        // Add variation if present
        if (rev.variations && rev.variations.trim() !== "") {
          try {
            const varPlayer = new SgfPlayer(rev.variations, boardSize);
            if (varPlayer.root) {
              // Ensure it's not already added
              const targetProps = JSON.stringify(varPlayer.root.properties);
              const alreadyExists = parentNode.children.some(child => {
                return JSON.stringify(child.properties) === targetProps;
              });

              // Store the DB Review ID on the variation node for edit tracking
              (varPlayer.root as any).review_id = rev.id;

              if (!alreadyExists) {
                // Attach comment directly onto the variation root node
                if (rev.comment && rev.comment.trim() !== "") {
                  if (!varPlayer.root.properties["C"]) {
                    varPlayer.root.properties["C"] = [];
                  }
                  varPlayer.root.properties["C"].push(`${rev.reviewer_name}: ${rev.comment}`);
                }

                markAsVariation(varPlayer.root, rev.reviewer_name);
                varPlayer.root.parent = parentNode;
                parentNode.children.push(varPlayer.root);
              } else {
                // If already exists, find existing and update attributes
                const existingChild = parentNode.children.find(child => JSON.stringify(child.properties) === targetProps);
                if (existingChild) {
                  (existingChild as any).review_id = rev.id;
                  if (rev.comment && rev.comment.trim() !== "") {
                    existingChild.properties["C"] = [`${rev.reviewer_name}: ${rev.comment}`];
                  }
                }
              }
            }
          } catch (e) {
            console.error("Failed to parse variation SGF:", e);
          }
        } else {
          // No variation, just comment on the main path node itself
          if (!parentNode.properties["C"]) {
            parentNode.properties["C"] = [];
          }
          const commentText = `${rev.reviewer_name}: ${rev.comment}`;
          if (!parentNode.properties["C"].includes(commentText)) {
            parentNode.properties["C"].push(commentText);
          }
          (parentNode as any).review_id = rev.id;
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
      // Find review ID associated with this node or parent (if we are in a variation, the review ID is on the variation root)
      let associatedReviewId = (state.node as any).review_id || "";
      let associatedReviewer = (state.node as any).reviewer_name || "";
      
      // If we don't have review_id on current node, traverse up to find it in the variation path
      if (!associatedReviewId && isViewingVariation) {
        let temp: SgfNode | null = state.node;
        while (temp) {
          if ((temp as any).review_id) {
            associatedReviewId = (temp as any).review_id;
            associatedReviewer = (temp as any).reviewer_name || "";
            break;
          }
          temp = temp.parent;
        }
      }

      for (const rawComment of state.node.properties["C"]) {
        const colonIndex = rawComment.indexOf(":");
        let author = "コメント";
        let text = rawComment;
        if (colonIndex !== -1) {
          author = rawComment.substring(0, colonIndex).trim();
          text = rawComment.substring(colonIndex + 1).trim();
        }
        
        comments.push({
          author: author,
          text: text,
          reviewId: associatedReviewId,
          reviewerName: associatedReviewer
        });
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
    // Filter out primary path nodes (only show actual variation review branches)
    alternativeBranches = player.getAlternativeBranches()
      .filter((node) => (node as any).is_variation === true)
      .map((node) => {
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

  // Save or Update Review Comment
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
      
      // Extract the variation SGF sub-tree from the branch start point
      let variationsSgf = "";
      let targetMoveNumber = currentIndex;
      const currentNode = state.node;
      
      if (currentNode && currentNode.parent) {
        // Find the root of this variation branch (whose parent is on the primary path)
        let variationRoot = currentNode;
        let temp = currentNode;
        while (temp && temp.parent) {
          if (!(temp.parent as any).is_variation) {
            variationRoot = temp;
            break;
          }
          temp = temp.parent;
        }

        if ((variationRoot as any).is_variation) {
          // Calculate the move number of the parent (primary path node)
          let count = 0;
          let p: SgfNode | null = player.root;
          while (p && p !== variationRoot.parent) {
            if (p.children.length > 0) {
              p = p.children[0];
              count++;
            } else {
              break;
            }
          }
          targetMoveNumber = count;
          variationsSgf = stringifySgf(variationRoot);
        }
      }

      let res: Response;
      const payload = {
        move_number: targetMoveNumber,
        node_path: String(targetMoveNumber),
        reviewer_name: reviewerName.trim(),
        comment: reviewComment.trim(),
        variations: variationsSgf
      };

      if (editingReviewId) {
        // Update existing review
        res = await fetch(`/api/kifus/${kifuId}/reviews/${editingReviewId}`, {
          method: 'PUT',
          headers: auth.getHeaders(),
          body: JSON.stringify(payload)
        });
      } else {
        // Create new review
        if (isPublicProfileMode) {
          res = await fetch(`/api/u/${userId}/kifus/${kifuId}/reviews`, {
            method: 'POST',
            headers: auth.getHeaders(),
            body: JSON.stringify(payload)
          });
        } else if (shareToken) {
          res = await fetch(`/api/share/${shareToken}/reviews`, {
            method: 'POST',
            headers: auth.getHeaders(),
            body: JSON.stringify(payload)
          });
        } else {
          res = await fetch(`/api/kifus/${kifuId}/reviews`, {
            method: 'POST',
            headers: auth.getHeaders(),
            body: JSON.stringify(payload)
          });
        }
      }

      if (!res.ok) {
        const errorData = await res.json().catch(() => ({}));
        throw new Error(errorData.error || "Failed to save review");
      }

      const savedReview = await res.json();
      
      if (editingReviewId) {
        // Replace in reviewList
        reviewList = reviewList.map(r => r.id === editingReviewId ? savedReview : r);
        editingReviewId = null;
      } else {
        reviewList = [...reviewList, savedReview];
      }

      // Re-initialize player with original SGF and re-merge all reviews to ensure clean state
      if (kifu) {
        player = new SgfPlayer(kifu.sgf_data, boardSize);
        mergeReviewsIntoPlayer();
        updatePlayerState();
      }

      // Reset comment text
      reviewComment = "";
      
      const M = getM();
      if (M) {
        M.toast({ html: '添削内容を保存しました！', classes: 'green darken-1' });
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

  function handleEditReview(reviewId: string, reviewerNameVal: string, commentVal: string) {
    editingReviewId = reviewId;
    reviewMode = true;
    reviewerName = reviewerNameVal;
    reviewComment = commentVal;

    // Navigate to the branch point
    const rev = reviewList.find(r => r.id === reviewId);
    if (rev && player) {
      player.goTo(rev.move_number);
      // Select the branch with the correct review_id
      const state = player.getCurrentState();
      if (state.node) {
        const branchIdx = state.node.children.findIndex(child => (child as any).review_id === reviewId);
        if (branchIdx !== -1) {
          player.selectBranch(branchIdx);
        }
      }
      updatePlayerState();
    }

    const M = getM();
    if (M) {
      M.toast({ html: '添削の編集を開始しました。内容や変化手順を修正して「更新」を押してください。', classes: 'amber darken-2' });
    }
  }

  function handleCancelEdit() {
    editingReviewId = null;
    reviewComment = "";
    
    // Re-initialize player to reset unsaved scratch changes
    if (kifu) {
      player = new SgfPlayer(kifu.sgf_data, boardSize);
      mergeReviewsIntoPlayer();
      updatePlayerState();
    }

    const M = getM();
    if (M) {
      M.toast({ html: '編集をキャンセルしました', classes: 'grey darken-1' });
    }
  }

  async function handleDeleteReview(reviewId: string) {
    if (!confirm("この指導コメントと変化図のまとまりを削除してもよろしいですか？")) return;

    try {
      const res = await fetch(`/api/kifus/${kifuId}/reviews/${reviewId}`, {
        method: 'DELETE',
        headers: auth.getHeaders()
      });

      if (!res.ok) {
        const errorData = await res.json().catch(() => ({}));
        throw new Error(errorData.error || "Failed to delete review");
      }

      // Filter out from local list
      reviewList = reviewList.filter(r => r.id !== reviewId);
      
      // Reload game tree
      if (kifu) {
        player = new SgfPlayer(kifu.sgf_data, boardSize);
        mergeReviewsIntoPlayer();
        updatePlayerState();
      }

      const M = getM();
      if (M) {
        M.toast({ html: '添削を削除しました', classes: 'green darken-1' });
      }
    } catch (err: any) {
      const M = getM();
      if (M) {
        M.toast({ html: 'エラー: ' + err.message, classes: 'red' });
      }
    }
  }

  function handleReturnToMainPath() {
    if (!player) return;

    const state = player.getCurrentState();
    let current = state.node;
    let targetNode: SgfNode | null = null;

    // Traverse up to find the first node that is not a variation (part of the primary path)
    while (current) {
      if (!(current as any).is_variation) {
        targetNode = current;
        break;
      }
      current = current.parent;
    }

    if (targetNode) {
      // Rebuild the primary path history completely
      player.initGame();
      
      // Find the target node inside the rebuilt primary path history
      const idx = player.history.findIndex(h => h.node === targetNode);
      if (idx !== -1) {
        player.goTo(idx);
      }
      updatePlayerState();

      const M = getM();
      if (M) {
        M.toast({ html: '本線（オリジナルの棋譜）に戻りました', classes: 'grey darken-3' });
      }
    }
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
      <button class="nm-btn-flat" on:click={onBack} style="padding-left: 0; display: inline-flex; align-items: center; gap: 6px;">
        <i class="material-icons">arrow_back</i>戻る
      </button>
      {#if kifu}
        <h5 style="margin: 0; font-weight: 600; margin-left: 1.5rem; color: var(--nm-accent);">{kifu.title}</h5>
      {/if}
    </div>
    {#if kifu && isOwner}
      <button class="nm-btn" on:click={() => showShareDialog = true}>
        <i class="material-icons" style="font-size: 1.2rem;">share</i>共有設定
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
      <div class="nm-card controls-panel" style="margin-top: 1.5rem;">
        <div class="card-content" style="padding: 16px 24px;">
          <!-- Slider -->
          <div class="range-field d-flex align-center" style="display: flex; align-items: center; margin-bottom: 0.75rem;">
            <span style="font-weight: 600; min-width: 60px; color: var(--nm-accent); font-size: 0.95rem;">{currentIndex} / {maxIndex}手</span>
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
          <div class="buttons-row" style="display: flex; justify-content: center; gap: 12px; flex-wrap: wrap; margin-top: 10px;">
            <button class="nm-btn-flat nm-btn-round" on:click={goFirst} title="最初へ">
              <i class="material-icons">first_page</i>
            </button>
            <button class="nm-btn-flat nm-btn-round" on:click={() => stepBack(10)} title="10手戻る">
              <i class="material-icons">fast_rewind</i>
            </button>
            <button class="nm-btn-flat nm-btn-round" on:click={() => stepBack(1)} title="1手戻る">
              <i class="material-icons">navigate_before</i>
            </button>
            <button class="nm-btn-primary nm-btn-round" style="width: 44px !important; height: 44px !important;" on:click={toggleAutoplay} title={isAutoplay ? '一時停止' : '自動再生'}>
              <i class="material-icons">{isAutoplay ? 'pause' : 'play_arrow'}</i>
            </button>
            <button class="nm-btn-flat nm-btn-round" on:click={() => stepForward(1)} title="1手進む">
              <i class="material-icons">navigate_next</i>
            </button>
            <button class="nm-btn-flat nm-btn-round" on:click={() => stepForward(10)} title="10手進む">
              <i class="material-icons">fast_forward</i>
            </button>
            <button class="nm-btn-flat nm-btn-round" on:click={goLast} title="最後へ">
              <i class="material-icons">last_page</i>
            </button>
          </div>

          {#if isViewingVariation}
            <!-- svelte-ignore a11y-missing-attribute -->
            <div class="animate-fade-in" style="margin-top: 16px; display: flex; justify-content: center;">
              <button class="nm-btn" on:click={handleReturnToMainPath} style="border-radius: 20px; font-weight: 500; display: inline-flex; align-items: center; gap: 6px; padding: 6px 16px;">
                <i class="material-icons" style="font-size: 1.15rem;">assignment_return</i>
                本線（元の棋譜）に戻る
              </button>
            </div>
          {/if}
        </div>
      </div>
    </div>

    <!-- Right Column: Game Info, Comments & Variations -->
    <div class="col s12 l6 text-left" style="text-align: left;">
      <!-- Game Info Card (Collapsible) -->
      {#if kifu}
      <div class="nm-card game-info-card hoverable" style="margin-top: 0; margin-bottom: 1.5rem;">
        <!-- Card Header Toggle (Click to Toggle) -->
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div class="card-header-toggle cursor-pointer" on:click={() => isGameInfoExpanded = !isGameInfoExpanded} style="padding: 14px 20px; display: flex; align-items: center; justify-content: space-between; cursor: pointer; user-select: none;">
          <div style="display: flex; align-items: center; gap: 12px; flex-wrap: wrap;">
            <span style="font-weight: 600; font-size: 1rem; display: inline-flex; align-items: center; gap: 6px; color: var(--nm-accent);">
              <i class="material-icons" style="font-size: 1.2rem;">info_outline</i>
              対局情報
            </span>
            <span style="font-size: 0.9rem; font-weight: 500; display: inline-flex; align-items: center; flex-wrap: wrap; gap: 6px; color: var(--nm-text-main);">
              <span class="font-weight-500">● {kifu.black_player || '不明'}</span>
              <span style="font-size: 0.8rem; color: var(--nm-text-muted);">(石 {captives.W})</span>
              <span style="font-size: 0.75rem; color: var(--nm-text-muted);">vs</span>
              <span class="font-weight-500">○ {kifu.white_player || '不明'}</span>
              <span style="font-size: 0.8rem; color: var(--nm-text-muted);">(石 {captives.B})</span>
              {#if kifu.result}
                <span class="nm-badge-inset" style="margin-left: 6px; font-size: 0.75rem; padding: 2px 8px; font-weight: 600; color: var(--nm-accent);">{kifu.result}</span>
              {/if}
            </span>
          </div>
          <button class="nm-btn-flat nm-btn-round" style="width: 32px; height: 32px; min-width: 32px; padding: 0;">
            <i class="material-icons" style="font-size: 1.3rem; transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1); transform: {isGameInfoExpanded ? 'rotate(180deg)' : 'rotate(0deg)'}; color: var(--nm-accent);">keyboard_arrow_down</i>
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
        <div class="nm-card animate-fade-in" style="margin-bottom: 1.5rem; border: 1px solid rgba(255, 152, 0, 0.3) !important;">
          <div class="card-content" style="padding: 16px 20px;">
            <span class="valign-wrapper orange-text text-darken-4" style="font-weight: 600; display: inline-flex; align-items: center; gap: 8px;">
              <i class="material-icons" style="font-size: 1.2rem;">call_split</i> 変化図があります
            </span>
            <div style="display: flex; gap: 10px; flex-wrap: wrap; margin-top: 10px;">
              <!-- Main branch return button if not on primary branch -->
              {#if isViewingVariation}
                <!-- svelte-ignore a11y-missing-attribute -->
                <button class="nm-btn" on:click={handleReturnToMainPath} style="display: inline-flex; align-items: center; gap: 4px; border-radius: 4px; font-weight: 500;">
                  <i class="material-icons" style="font-size: 1.1rem;">assignment_return</i>
                  本線に戻る
                </button>
              {/if}
              <!-- Display other branch buttons -->
              {#each alternativeBranches as branch}
                <button class="nm-btn" on:click={() => selectBranch(branch.originalIndex)}>
                  {branch.label} に切り替え
                </button>
              {/each}
            </div>
          </div>
        </div>
      {/if}

      <!-- Comments / Review Card -->
      <div class="nm-card" style="min-height: 180px; display: flex; flex-direction: column; margin-bottom: 1.5rem;">
        <div class="card-content" style="padding: 20px; flex-grow: 1;">
          <span class="card-title" style="font-size: 1.15rem; font-weight: 600; margin-bottom: 12px; color: var(--nm-accent);">
            検討・指導コメント (第 {currentIndex} 手)
          </span>

          {#if comments.length === 0}
            <div class="center-align grey-text text-lighten-1" style="padding: 2.5rem 0;">
              <i class="material-icons" style="font-size: 3rem; color: var(--nm-text-muted); opacity: 0.6;">chat_bubble_outline</i>
              <p style="margin: 8px 0 0 0; font-size: 0.9rem; color: var(--nm-text-muted);">この手に対するコメントはありません</p>
            </div>
          {:else}
            <div class="comments-list" style="max-height: 250px; overflow-y: auto; padding-right: 4px;">
              {#each comments as comment}
                <div class="comment-item" style="border-bottom: 1px solid rgba(163, 177, 198, 0.15); padding: 12px 0; display: flex; justify-content: space-between; align-items: flex-start;">
                  <div style="flex-grow: 1; text-align: left;">
                    <span class="nm-badge" style="font-weight: 600; margin-bottom: 6px; padding: 2px 10px; font-size: 0.8rem; color: var(--nm-accent);">
                      {comment.author}
                    </span>
                    <p style="margin: 0; padding-left: 4px; font-size: 0.95rem; white-space: pre-wrap; color: var(--nm-text-main); line-height: 1.5;">{comment.text}</p>
                  </div>
                  {#if comment.reviewId && (isOwner || auth.username === comment.reviewerName)}
                    <div style="display: flex; gap: 6px; margin-left: 12px;">
                      <!-- svelte-ignore a11y-missing-attribute -->
                      <button class="nm-btn-flat nm-btn-round" on:click={() => handleEditReview(comment.reviewId || '', comment.reviewerName || '', comment.text)} title="指導を編集" style="width: 28px; height: 28px; min-width: 28px; padding: 0;">
                        <i class="material-icons" style="font-size: 1.1rem; color: var(--nm-accent);">edit</i>
                      </button>
                      <!-- svelte-ignore a11y-missing-attribute -->
                      <button class="nm-btn-flat nm-btn-round" on:click={() => handleDeleteReview(comment.reviewId || '')} title="指導を削除" style="width: 28px; height: 28px; min-width: 28px; padding: 0;">
                        <i class="material-icons" style="font-size: 1.1rem; color: #e53935;">delete</i>
                      </button>
                    </div>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <!-- Mode Toggle & Edit Panel Footer -->
        <div class="card-action" style="border-top: 1px solid rgba(163, 177, 198, 0.2); padding: 16px 20px; border-radius: 0 0 var(--nm-radius-md) var(--nm-radius-md);">
          <div class="switch" style="margin-bottom: 12px;">
            <label class="black-text" style="font-weight: 500; display: inline-flex; align-items: center; gap: 8px;">
              <span style="color: var(--nm-text-muted);">通常再生</span>
              <input type="checkbox" bind:checked={reviewMode}>
              <span class="lever brown lighten-3"></span>
              <span style="color: var(--nm-text-main); font-weight: 600;">添削モード (盤面入力可)</span>
            </label>
          </div>

          {#if reviewMode}
            <div class="review-edit-panel animate-fade-in" style="margin-top: 12px; border-top: 1px dashed rgba(163, 177, 198, 0.2); padding-top: 16px;">
              <p style="font-size: 0.85rem; margin: 0 0 14px 0; color: var(--nm-text-muted); display: inline-flex; align-items: center; gap: 6px;">
                <i class="material-icons" style="font-size: 1.05rem;">info</i>
                碁盤をクリックして石を置くと変化図を作成できます。解説コメントを入力してください。
              </p>
              
              <div class="row" style="margin-bottom: 0; display: flex; flex-direction: column; gap: 10px;">
                <div class="input-field col s12" style="margin-top: 0; margin-bottom: 0;">
                  <input id="reviewer_name" type="text" class="nm-input" bind:value={reviewerName} placeholder="添削者名" style="margin-bottom: 0;" />
                  <label for="reviewer_name" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem;">添削者</label>
                </div>
                <div class="input-field col s12" style="margin-top: 4px; margin-bottom: 0;">
                  <input id="review_comment" type="text" class="nm-input" bind:value={reviewComment} placeholder="指導・変化図の解説を入力してください" style="margin-bottom: 0;" />
                  <label for="review_comment" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem;">コメント</label>
                </div>
                <div class="col s12 right-align" style="display: flex; justify-content: flex-end; gap: 10px; margin-top: 10px;">
                  {#if editingReviewId}
                    <button class="nm-btn-flat" on:click={handleCancelEdit} disabled={isAddingReview}>
                      キャンセル
                    </button>
                    <button class="nm-btn-primary" disabled={!reviewerName || !reviewComment || isAddingReview} on:click={handleSaveReview}>
                      <i class="material-icons" style="font-size: 1.2rem;">save</i>添削を更新
                    </button>
                  {:else}
                    <button class="nm-btn-primary" disabled={!reviewerName || !reviewComment || isAddingReview} on:click={handleSaveReview}>
                      <i class="material-icons" style="font-size: 1.2rem;">save</i>添削を保存
                    </button>
                  {/if}
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
