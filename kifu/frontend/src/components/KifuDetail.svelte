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
      <button class="nm-btn-flat" onclick={onBack} style="padding-left: 0; display: inline-flex; align-items: center; gap: 6px;">
        <i class="material-icons">arrow_back</i>戻る
      </button>
      {#if kifu}
        <h5 style="margin: 0; font-weight: 600; margin-left: 1.5rem; color: var(--wc-text); font-family: 'Shippori Mincho B1', serif; letter-spacing: 0.02em;">{kifu.title}</h5>
      {/if}
    </div>
    {#if kifu && isOwner}
      <button class="nm-btn" onclick={() => showShareDialog = true}>
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
          <div class="wc-variation-badge animate-fade-in" style="position: absolute; top: 15px; left: 15px; z-index: 10;">
            <i class="material-icons" style="font-size: 0.9rem; vertical-align: middle;">call_split</i>
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
            <span style="font-weight: 600; min-width: 70px; color: var(--wc-accent); font-size: 0.9rem; font-family: 'JetBrains Mono', monospace;">{currentIndex} / {maxIndex}手</span>
            <input
              type="range"
              min="0"
              max={maxIndex}
              value={currentIndex}
              oninput={handleSliderChange}
              style="margin: 0 15px; flex-grow: 1;"
            />
          </div>

          <!-- Buttons Row -->
          <div class="buttons-row" style="display: flex; justify-content: center; gap: 12px; flex-wrap: wrap; margin-top: 10px;">
            <button class="nm-btn-flat nm-btn-round" onclick={goFirst} title="最初へ">
              <i class="material-icons">first_page</i>
            </button>
            <button class="nm-btn-flat nm-btn-round" onclick={() => stepBack(10)} title="10手戻る">
              <i class="material-icons">fast_rewind</i>
            </button>
            <button class="nm-btn-flat nm-btn-round" onclick={() => stepBack(1)} title="1手戻る">
              <i class="material-icons">navigate_before</i>
            </button>
            <button class="nm-btn-primary nm-btn-round" style="width: 44px !important; height: 44px !important;" onclick={toggleAutoplay} title={isAutoplay ? '一時停止' : '自動再生'}>
              <i class="material-icons">{isAutoplay ? 'pause' : 'play_arrow'}</i>
            </button>
            <button class="nm-btn-flat nm-btn-round" onclick={() => stepForward(1)} title="1手進む">
              <i class="material-icons">navigate_next</i>
            </button>
            <button class="nm-btn-flat nm-btn-round" onclick={() => stepForward(10)} title="10手進む">
              <i class="material-icons">fast_forward</i>
            </button>
            <button class="nm-btn-flat nm-btn-round" onclick={goLast} title="最後へ">
              <i class="material-icons">last_page</i>
            </button>
          </div>

          {#if isViewingVariation}
            <!-- svelte-ignore a11y_missing_attribute -->
            <div class="animate-fade-in" style="margin-top: 16px; display: flex; justify-content: center;">
              <button class="nm-btn" onclick={handleReturnToMainPath} style="border-radius: 20px; font-weight: 500; display: inline-flex; align-items: center; gap: 6px; padding: 6px 16px;">
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
      <div class="em-magazine-card game-info-card hoverable" style="margin-top: 0; margin-bottom: 1.5rem;">
        <!-- Card Header Toggle (Click to Toggle) -->
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div class="card-header-toggle cursor-pointer" onclick={() => isGameInfoExpanded = !isGameInfoExpanded} style="padding: 16px 20px; display: flex; align-items: center; justify-content: space-between; cursor: pointer; user-select: none;">
          <div style="display: flex; align-items: center; gap: 12px; flex-wrap: wrap;">
            <span style="font-weight: 600; font-size: 1rem; display: inline-flex; align-items: center; gap: 6px; color: var(--wc-accent); font-family: 'Shippori Mincho B1', serif;">
              <i class="material-icons" style="font-size: 1.2rem;">info_outline</i>
              対局情報 — Record Specifications
            </span>
          </div>
          <button class="nm-btn-flat nm-btn-round" style="width: 32px; height: 32px; min-width: 32px; padding: 0;">
            <i class="material-icons" style="font-size: 1.3rem; transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1); transform: {isGameInfoExpanded ? 'rotate(180deg)' : 'rotate(0deg)'}; color: var(--wc-accent);">keyboard_arrow_down</i>
          </button>
        </div>

        <!-- Expanded details -->
        {#if isGameInfoExpanded}
          <div class="card-content" style="padding: 0 20px 20px 20px; border-top: 1px dashed var(--wc-border); transition: all 0.3s ease;">
            <table class="em-table" style="margin-top: 12px;">
              <tbody>
                <tr>
                  <th>● 黒番 (PB)</th>
                  <td>
                    <span style="font-weight: 600; color: var(--wc-text);">{kifu.black_player || '不明'}</span>
                    {#if kifu.black_rank}
                      <span class="wc-tag" style="margin-left: 6px; font-size: 0.72rem; padding: 1px 6px;">{kifu.black_rank}</span>
                    {/if}
                    <div style="font-size: 0.8rem; color: var(--wc-text-muted); margin-top: 2px;">
                      アゲハマ: <strong style="font-family: 'JetBrains Mono', monospace; font-size: 0.85rem;">{captives.W}</strong> 子
                    </div>
                  </td>
                </tr>
                <tr>
                  <th>○ 白番 (PW)</th>
                  <td>
                    <span style="font-weight: 600; color: var(--wc-text);">{kifu.white_player || '不明'}</span>
                    {#if kifu.white_rank}
                      <span class="wc-tag" style="margin-left: 6px; font-size: 0.72rem; padding: 1px 6px;">{kifu.white_rank}</span>
                    {/if}
                    <div style="font-size: 0.8rem; color: var(--wc-text-muted); margin-top: 2px;">
                      アゲハマ: <strong style="font-family: 'JetBrains Mono', monospace; font-size: 0.85rem;">{captives.B}</strong> 子
                    </div>
                  </td>
                </tr>
                <tr>
                  <th>コミ / 置き石</th>
                  <td style="font-family: 'JetBrains Mono', monospace;">コミ: {kifu.komi}目 / {kifu.handicap}子</td>
                </tr>
                <tr>
                  <th>対局結果</th>
                  <td style="font-weight: 600; color: var(--wc-accent-warm);">{kifu.result || '対局中'}</td>
                </tr>
                <tr>
                  <th>対局日</th>
                  <td>{kifu.game_date || '未登録'}</td>
                </tr>
              </tbody>
            </table>
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
                <!-- svelte-ignore a11y_missing_attribute -->
                <button class="nm-btn" onclick={handleReturnToMainPath} style="display: inline-flex; align-items: center; gap: 4px; border-radius: 4px; font-weight: 500;">
                  <i class="material-icons" style="font-size: 1.1rem;">assignment_return</i>
                  本線に戻る
                </button>
              {/if}
              <!-- Display other branch buttons -->
              {#each alternativeBranches as branch}
                <button class="nm-btn" onclick={() => selectBranch(branch.originalIndex)}>
                  {branch.label} に切り替え
                </button>
              {/each}
            </div>
          </div>
        </div>
      {/if}

      <!-- Comments / Review Card -->
      <div class="em-magazine-card" style="min-height: 180px; display: flex; flex-direction: column; margin-bottom: 1.5rem;">
        <div class="card-content" style="padding: 24px; flex-grow: 1;">
          <span class="card-title" style="font-size: 1.1rem; font-weight: 600; margin-bottom: 16px; color: var(--wc-accent); font-family: 'Shippori Mincho B1', serif; letter-spacing: 0.02em;">
            検討・指導コメント — Editorial Reviews (第 {currentIndex} 手)
          </span>

          {#if comments.length === 0}
            <div class="center-align" style="padding: 3rem 0; opacity: 0.5;">
              <i class="material-icons" style="font-size: 2.8rem; color: var(--wc-text-muted);">chat_bubble_outline</i>
              <p style="margin: 8px 0 0 0; font-size: 0.88rem; color: var(--wc-text-muted); font-family: 'DM Sans', sans-serif;">この手に対するコメントはありません</p>
            </div>
          {:else}
            <div class="comments-list" style="max-height: 250px; overflow-y: auto; padding-right: 4px;">
              {#each comments as comment}
                <div class="comment-item" style="border-bottom: 1px dashed var(--wc-border); padding: 14px 0; display: flex; justify-content: space-between; align-items: flex-start;">
                  <div style="flex-grow: 1; text-align: left;">
                    <span class="wc-tag" style="font-weight: 600; margin-bottom: 8px; padding: 2px 10px; font-size: 0.72rem; letter-spacing: 0.05em;">
                      {comment.author}
                    </span>
                    <p style="margin: 0; padding-left: 2px; font-size: 0.93rem; white-space: pre-wrap; color: var(--wc-text); line-height: 1.6; font-family: 'DM Sans', 'Noto Sans JP', sans-serif;">{comment.text}</p>
                  </div>
                  {#if comment.reviewId && (isOwner || auth.username === comment.reviewerName)}
                    <div style="display: flex; gap: 6px; margin-left: 12px;">
                      <!-- svelte-ignore a11y_missing_attribute -->
                      <button class="nm-btn-flat nm-btn-round" onclick={() => handleEditReview(comment.reviewId || '', comment.reviewerName || '', comment.text)} title="指導を編集" style="width: 28px; height: 28px; min-width: 28px; padding: 0;">
                        <i class="material-icons" style="font-size: 1.1rem; color: var(--wc-accent);">edit</i>
                      </button>
                      <!-- svelte-ignore a11y_missing_attribute -->
                      <button class="nm-btn-flat nm-btn-round" onclick={() => handleDeleteReview(comment.reviewId || '')} title="指導を削除" style="width: 28px; height: 28px; min-width: 28px; padding: 0;">
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
        <div class="card-action" style="border-top: 1px solid var(--wc-border); padding: 16px 24px; background: rgba(245, 240, 232, 0.35);">
          <div class="switch" style="margin-bottom: 12px;">
            <label style="font-weight: 500; display: inline-flex; align-items: center; gap: 8px; color: var(--wc-text); font-family: 'DM Sans', 'Noto Sans JP', sans-serif;">
              <span style="color: var(--wc-text-muted); font-size: 0.9rem;">通常再生</span>
              <input type="checkbox" bind:checked={reviewMode}>
              <span class="lever brown lighten-3"></span>
              <span style="color: var(--wc-text); font-weight: 600; font-size: 0.9rem;">添削モード (盤面入力可)</span>
            </label>
          </div>

          {#if reviewMode}
            <div class="review-edit-panel animate-fade-in" style="margin-top: 12px; border-top: 1px dashed var(--wc-border); padding-top: 16px;">
              <p style="font-size: 0.82rem; margin: 0 0 16px 0; color: var(--wc-text-muted); display: inline-flex; align-items: center; gap: 6px; font-family: 'DM Sans', 'Noto Sans JP', sans-serif; line-height: 1.4;">
                <i class="material-icons" style="font-size: 1rem; color: var(--wc-accent-warm);">info_outline</i>
                盤面をクリックして石を置くと変化図を作成できます。解説コメントを入力してください。
              </p>
              
              <div class="row" style="margin-bottom: 0; display: flex; flex-direction: column; gap: 12px;">
                <div class="input-field col s12" style="margin-top: 0; margin-bottom: 0;">
                  <input id="reviewer_name" type="text" class="nm-input" bind:value={reviewerName} placeholder="添削者名を入力" style="margin-bottom: 0;" />
                  <label for="reviewer_name" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text-muted);">添削者 (Reviewer)</label>
                </div>
                <div class="input-field col s12" style="margin-top: 4px; margin-bottom: 0;">
                  <input id="review_comment" type="text" class="nm-input" bind:value={reviewComment} placeholder="指導・変化図の解説を入力" style="margin-bottom: 0;" />
                  <label for="review_comment" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text-muted);">コメント (Comment)</label>
                </div>
                <div class="col s12 right-align" style="display: flex; justify-content: flex-end; gap: 10px; margin-top: 10px;">
                  {#if editingReviewId}
                    <button class="nm-btn-flat" onclick={handleCancelEdit} disabled={isAddingReview}>
                      キャンセル
                    </button>
                    <button class="nm-btn-primary" disabled={!reviewerName || !reviewComment || isAddingReview} onclick={handleSaveReview}>
                      <i class="material-icons" style="font-size: 1.2rem;">save</i>添削を更新
                    </button>
                  {:else}
                    <button class="nm-btn-primary" disabled={!reviewerName || !reviewComment || isAddingReview} onclick={handleSaveReview}>
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
    background: var(--wc-bg);
  }
  .comments-list::-webkit-scrollbar-thumb {
    background: var(--wc-mid);
    border-radius: 3px;
  }
  .comments-list {
    scrollbar-width: thin;
  }

  /* Playback controls buttons styling */
  .buttons-row button {
    background-color: var(--wc-surface) !important;
    color: var(--wc-text) !important;
    transition: var(--wc-transition-fast) !important;
    border: 1px solid var(--wc-border) !important;
    box-shadow: var(--nm-shadow-outset-sm) !important;
    display: inline-flex !important;
    align-items: center;
    justify-content: center;
  }

  /* Hover micro-animations */
  .buttons-row button:hover {
    background-color: var(--wc-surface-alt) !important;
    color: var(--wc-accent) !important;
    transform: scale(1.08) translateY(-1px) !important;
    box-shadow: var(--nm-shadow-outset-sm-hover) !important;
  }

  .buttons-row button:active {
    transform: scale(0.97) translateY(0) !important;
    box-shadow: var(--nm-shadow-inset) !important;
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

  /* Washi Clay Variation Badge */
  .wc-variation-badge {
    background: var(--wc-surface-alt);
    border: 1px solid var(--wc-accent-warm);
    padding: 4px 12px;
    border-radius: var(--wc-radius-sm);
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--wc-accent-warm);
    display: inline-flex;
    align-items: center;
    gap: 4px;
    box-shadow: var(--nm-shadow-outset-sm);
    pointer-events: none;
    font-family: 'DM Sans', 'Noto Sans JP', sans-serif;
  }
</style>
