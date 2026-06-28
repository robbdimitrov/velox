<script lang="ts">
  import { onMount } from 'svelte';
  import type { Seat } from '$lib/api/types';

  let {
    seats,
    selectedSeatIDs,
    onToggle
  }: {
    seats: Seat[];
    selectedSeatIDs: Set<string>;
    onToggle: (seat: Seat) => void;
  } = $props();

  let canvas = $state<HTMLCanvasElement>();
  let width = $state(700);
  let height = $state(360);

  const colors: Record<string, string> = {
    AVAILABLE: '#9CA3AF', // inkMuted
    SELECTED: '#7C3AED',  // primary
    HELD: '#FF2A5F',      // accent
    SOLD: '#15151A',      // panel
    UNKNOWN: '#272730'    // line
  };

  function draw() {
    if (!canvas) return;
    const context = canvas.getContext('2d');
    if (!context) return;

    context.clearRect(0, 0, width, height);
    
    // Background
    context.fillStyle = 'rgba(9, 9, 14, 0.4)';
    context.fillRect(0, 0, width, height);

    for (const seat of seats) {
      const selected = selectedSeatIDs.has(seat.seat_id);
      context.beginPath();
      context.arc(seat.x, seat.y, 6, 0, Math.PI * 2);
      
      // Add subtle glow to selected or held seats
      if (selected || seat.status === 'HELD') {
        context.shadowColor = selected ? 'rgba(124, 58, 237, 0.8)' : 'rgba(255, 42, 95, 0.8)';
        context.shadowBlur = 10;
      } else {
        context.shadowBlur = 0;
      }

      context.fillStyle = selected ? colors.SELECTED : colors[seat.status];
      context.fill();
      
      context.shadowBlur = 0; // Reset shadow for stroke
      context.lineWidth = selected ? 2 : 1;
      context.strokeStyle = selected
        ? '#F3F4F6'
        : seat.status === 'UNKNOWN'
          ? '#4B5563'
          : 'rgba(255,255,255,0.1)';
      context.stroke();
    }
  }

  function handleClick(event: MouseEvent) {
    if (!canvas) return;
    const rect = canvas.getBoundingClientRect();
    const x = ((event.clientX - rect.left) / rect.width) * width;
    const y = ((event.clientY - rect.top) / rect.height) * height;
    const hit = seats.find((seat) => Math.hypot(seat.x - x, seat.y - y) <= 9);
    if (hit) onToggle(hit);
  }

  onMount(() => {
    draw();
  });

  $effect(() => {
    // Explicitly read reactive dependencies to trigger the effect
    selectedSeatIDs.size;
    if (seats && canvas) {
      draw();
    }
  });
</script>

<div class="relative min-h-[360px] rounded-2xl border border-white/10 bg-black/40 shadow-lg overflow-hidden backdrop-blur-md">
  <canvas
    bind:this={canvas}
    class="h-full min-h-[360px] w-full cursor-crosshair transition-opacity duration-300 hover:opacity-90"
    {width}
    {height}
    onclick={handleClick}
    aria-label="Interactive seat map"
  ></canvas>
  <svg
    class="pointer-events-none absolute inset-0 h-full w-full opacity-60"
    viewBox="0 0 700 360"
    aria-hidden="true"
  >
    <path
      d="M18 16H682V342H18Z"
      fill="none"
      stroke="rgba(255,255,255,0.1)"
      stroke-width="2"
      rx="12"
    />
    <path d="M210 20V340M490 20V340" stroke="rgba(255,255,255,0.05)" stroke-width="1" stroke-dasharray="4 4" />
    <text x="350" y="28" fill="#F3F4F6" font-size="12" font-weight="bold" font-family="Outfit" letter-spacing="2" text-anchor="middle"
      >SECTION A</text
    >
    <text x="350" y="334" fill="#9CA3AF" font-size="11" font-weight="bold" font-family="Outfit" letter-spacing="4" text-anchor="middle"
      >STAGE</text
    >
  </svg>
</div>
