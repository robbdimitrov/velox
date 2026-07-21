# Design System

Velox uses a polished, image-less Bauhaus/Apollo interface for reservation-only
event access. Visual state must communicate availability, ownership, and risk
quickly in both light and dark themes.

## Tokens

| Token | Hex | Use |
| --- | --- | --- |
| Porcelain | `#F7F5F0` | Light app background. |
| Ink | `#101114` | Light primary text and dark app background. |
| Graphite | `#26282D` | Dark panels and light-theme text emphasis. |
| Mist | `#E4E0D8` | Light borders and disabled surfaces. |
| Lunar | `#F9FAFB` | Light work surfaces. |
| Apollo navy | `#172033` | Secondary control emphasis. |
| Deep red | `#8F1D2C` | Primary action, selected seats, focus accent. |
| Signal amber | `#C47A1C` | Warnings, expiring reservations, stale state. |
| Control green | `#2E7D5B` | Confirmed reservation state. |
| Urgency red | `#C62828` | Errors, destructive actions, held-by-other state. |

Primary actions and selected-seat state use deep red. Every token pair must
meet WCAG AA contrast in both light and dark DaisyUI themes.

## Typography

- Use system fonts only. Do not import remote or bundled display fonts.
- Use tabular numerals for timers, seat IDs, counters, and operational values.
- Keep dashboard and form headings compact. Reserve hero-scale type for the
  public discovery first viewport.
- Letter spacing remains normal except deliberate uppercase labels.

## Layout

- Global shell owns page width, viewport inset, and nav-to-content spacing.
- Use `max-w-7xl` for discovery, seat maps, wallet, and dashboards.
- Use `max-w-5xl` for reservation review flows.
- Use `max-w-3xl` for setup forms.
- Use `max-w-md` for auth.
- Major panel groups use a 6-unit gap.
- Cards are for repeated objects or modal-like tools. Do not nest cards inside
  cards.

## Components

- Implement theming with Tailwind v4 and DaisyUI 5 tokens. Theme variants are
  System, Light, and Dark; logout clears the local theme preference.
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
| Selected | Deep red fill and outline | Toggleable by current user. |
| Held | Urgency red emphasis | Not selectable unless held by current reservation. |
| Confirmed | Control green or solid muted node | Not selectable. |
| Cancelled | Urgency red outline or disabled red-tinted node | Not selectable. |
| Unavailable | Line-blue disabled node | Not selectable. |
| Unknown | Outlined node with muted fill | Disabled until refreshed. |
| Stale | Normal state plus stale indicator on section/map | Risky actions should freeze when lag is high. |

Seat maps must have stable dimensions and fixed node sizing so hover, status,
and selection changes do not resize the layout.

## Organizer Patterns

- Dashboards optimize for scanning: compact metric rows, inventory status,
  projection lag, reservation summaries, and operational actions.
- Destructive actions, especially event cancellation, must use urgency color
  and explicit confirmation.
- Staff controls are absent or disabled until membership assignment is real.
- Metrics must not silently use random fallback values in final showcase state.

## States

Empty states:

- Discovery: no events matching filters, with filters still visible.
- Wallet: no reservation tickets yet, with no transfer/scanner controls.
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

- Event discovery and detail screens are image-less. Use typography, geometry,
  color, motion restraint, and live state instead of event artwork.
- README screenshots must come from the actual local app after the route and
  data contracts support the showcased flows.

## Future Controls

Transfer, scanner, upgrade, and staff invite controls must be active
only when the backing route, persistence, and tests exist. Otherwise render the
state as unavailable or omit the control.
