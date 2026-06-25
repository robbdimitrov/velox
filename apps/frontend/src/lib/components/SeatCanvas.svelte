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

  let canvas: HTMLCanvasElement;
  let width = $state(700);
  let height = $state(360);

  const colors: Record<string, string> = {
    AVAILABLE: '#7A7A86',
    SELECTED: '#5533FF',
    HELD: '#FF3366',
    SOLD: '#0F0F11',
    UNKNOWN: '#2A2A31'
  };

  function draw() {
    const context = canvas?.getContext('2d');
    if (!context) return;

    context.clearRect(0, 0, width, height);
    context.fillStyle = '#111116';
    context.fillRect(0, 0, width, height);

    for (const seat of seats) {
      const selected = selectedSeatIDs.has(seat.seat_id);
      context.beginPath();
      context.arc(seat.x, seat.y, 6, 0, Math.PI * 2);
      context.fillStyle = selected ? colors.SELECTED : colors[seat.status];
      context.fill();
      context.lineWidth = selected ? 2 : 1;
      context.strokeStyle = selected ? '#D7D7DE' : seat.status === 'UNKNOWN' ? '#7A7A86' : '#2A2A31';
      context.stroke();
    }
  }

  function handleClick(event: MouseEvent) {
    const rect = canvas.getBoundingClientRect();
    const x = ((event.clientX - rect.left) / rect.width) * width;
    const y = ((event.clientY - rect.top) / rect.height) * height;
    const hit = seats.find((seat) => Math.hypot(seat.x - x, seat.y - y) <= 9);
    if (hit) onToggle(hit);
  }

  onMount(draw);

  $effect(draw);
</script>

<div class="relative min-h-[360px] border border-line bg-carbon">
  <canvas
    bind:this={canvas}
    class="h-full min-h-[360px] w-full cursor-crosshair"
    {width}
    {height}
    onclick={handleClick}
    aria-label="Interactive seat map"
  ></canvas>
  <svg class="pointer-events-none absolute inset-0 h-full w-full" viewBox="0 0 700 360" aria-hidden="true">
    <path d="M18 16H682V342H18Z" fill="none" stroke="#2A2A31" stroke-width="2" />
    <path d="M210 20V340M490 20V340" stroke="#2A2A31" stroke-width="1" />
    <text x="350" y="28" fill="#D7D7DE" font-size="12" text-anchor="middle">SECTION A</text>
    <text x="350" y="334" fill="#D7D7DE" font-size="11" text-anchor="middle">STAGE</text>
  </svg>
</div>
