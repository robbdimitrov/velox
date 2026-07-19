# Design System

Velox uses a dense command-center interface for live entertainment and flash
sale operations. Visual state must communicate risk and ownership quickly.

## Tokens

| Token | Hex | Use |
| --- | --- | --- |
| Carbon | `#07080B` | App background and sold seat body. |
| Panel | `#121722` | Primary work surfaces. |
| Panel soft | `#1A2130` | Secondary rows and selected panel backgrounds. |
| Line blue | `#273244` | Borders, section outlines, disabled controls. |
| Warm ink | `#F7F1E8` | Primary readable foreground. |
| Muted slate | `#8FA3B8` | Secondary text and metadata. |
| Signal amber | `#F2B84B` | Primary action, selected seats, active filters. |
| Electric teal | `#39D6C8` | Live accent and secondary operational emphasis. |
| Urgency red | `#FF5C5C` | Errors, destructive actions, holds by others, cancellation. |

Selected-seat and primary-action color is signal amber. Do not use indigo for
selected seats unless the implementation tokens are changed in the same
release.

## Typography

- Use Space Grotesk for UI text.
- Use Space Mono for timers, prices, seat IDs, counters, and operational
  numerals.
- Keep dashboard and form headings compact. Reserve hero-scale type for the
  public discovery first viewport.
- Letter spacing remains normal except deliberate uppercase labels.

## Layout

- Global shell owns page width, viewport inset, and nav-to-content spacing.
- Use `max-w-7xl` for discovery, seat maps, wallet, and dashboards.
- Use `max-w-5xl` for review and checkout flows.
- Use `max-w-3xl` for setup forms.
- Use `max-w-md` for auth.
- Major panel groups use a 6-unit gap.
- Cards are for repeated objects or modal-like tools. Do not nest cards inside
  cards.

## Components

- Buttons use DaisyUI button primitives plus Lucide icons for clear actions.
- Forms use labeled DaisyUI inputs, textareas, selects, and error strips.
- Panels use sharp 0 to 4 px radii and visible line borders.
- Tabs and segmented controls represent filters or modes.
- Sliders or steppers are reserved for numeric controls; do not use plain text
  pills for tool controls where a standard control exists.
- Live indicators should be fixed-size so changing numbers do not shift layout.

## Seat States

| State | Visual treatment | Interaction |
| --- | --- | --- |
| Available | Muted grey node with line border | Selectable. |
| Selected | Signal amber fill and outline | Toggleable by current user. |
| Held | Urgency red/crimson emphasis | Not selectable unless held by current reservation. |
| Sold | Solid carbon/dim node | Not selectable. |
| Cancelled | Urgency red outline or disabled red-tinted node | Not selectable. |
| Unavailable | Line-blue disabled node | Not selectable. |
| Unknown | Outlined node with muted fill | Disabled until refreshed. |
| Stale | Normal state plus stale indicator on section/map | Risky actions should freeze when lag is high. |

Seat maps must have stable dimensions and fixed node sizing so hover, status,
and selection changes do not resize the layout.

## Organizer Patterns

- Dashboards optimize for scanning: compact metric rows, inventory status,
  projection lag, order summaries, and operational actions.
- Destructive actions, especially event cancellation, must use urgency color
  and explicit confirmation.
- Staff controls are absent or disabled until membership assignment is real.
- Metrics must not silently use random fallback values in final showcase state.

## States

Empty states:

- Discovery: no events matching filters, with filters still visible.
- Wallet: no tickets yet, with no transfer/scanner controls.
- Organizer venues/events: clear create action if user has organizer role.

Error states:

- Use a single concise error strip near the failed control.
- Preserve typed input after validation failures.
- Do not expose backend internals, SQL, broker, or token details.

Degraded states:

- Show projection lag and stale snapshots where backend provides it.
- Disable reservation actions for cancelled or non-bookable events.
- Rate-limit responses should show waiting-room behavior, not generic failure.

## Mobile

- Discovery filters collapse into stacked controls.
- Seat map keeps fixed aspect constraints and moves selected-seat summary below
  the map when side-by-side space is unavailable.
- Organizer dashboards become one-column but preserve metric hierarchy.
- Button labels must fit without truncating critical action words.

## Assets And Screenshots

- Event imagery must come from backend image keys or documented local assets.
- Hardcoded event ID to image mapping is a temporary frontend seam.
- README screenshots must come from the actual local app after the route and
  data contracts support the showcased flows.

## Future Controls

Transfer, scanner, upgrade, staff invite, and payment controls must be active
only when the backing route, persistence, and tests exist. Otherwise render the
state as unavailable or omit the control.
