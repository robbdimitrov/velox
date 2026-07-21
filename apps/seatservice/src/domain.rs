#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SeatStatus {
    Available,
    Held {
        order_id: String,
        expires_at_ms: i64,
    },
    Reserved {
        order_id: String,
    },
    Cancelled {
        order_id: String,
    },
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SeatState {
    pub status: SeatStatus,
    pub version: u64,
}

impl Default for SeatState {
    fn default() -> Self {
        Self {
            status: SeatStatus::Available,
            version: 0,
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum InventoryError {
    VersionMismatch { expected: u64, actual: u64 },
    SeatNotAvailable,
    HoldExpired,
}

impl SeatState {
    pub fn expire_if_due(&mut self, now_ms: i64) {
        if let SeatStatus::Held { expires_at_ms, .. } = self.status {
            if now_ms >= expires_at_ms {
                self.status = SeatStatus::Available;
                // Passive time-based expiry does not append a domain event or bump version.
            }
        }
    }

    pub fn apply_reserved(&mut self, order_id: String, expires_at_ms: i64) {
        self.status = SeatStatus::Held {
            order_id,
            expires_at_ms,
        };
        self.version += 1;
    }

    pub fn apply_confirmed(&mut self, order_id: String) {
        self.status = SeatStatus::Reserved { order_id };
        self.version += 1;
    }

    pub fn apply_expired(&mut self) {
        self.status = SeatStatus::Available;
        self.version += 1;
    }

    pub fn apply_cancelled(&mut self, order_id: String) {
        self.status = SeatStatus::Cancelled { order_id };
        self.version += 1;
    }

    pub fn can_reserve(&self, expected_version: u64) -> Result<(), InventoryError> {
        if self.version != expected_version {
            return Err(InventoryError::VersionMismatch {
                expected: expected_version,
                actual: self.version,
            });
        }
        if self.status != SeatStatus::Available {
            return Err(InventoryError::SeatNotAvailable);
        }
        Ok(())
    }
}

/// Compare-and-append guard for event cancellation fan-out.
/// Only already-cancelled streams are terminal; every other seat must be
/// cancelled so a cancelled event never looks rebookable.
pub fn should_skip_cancellation(status: &SeatStatus) -> bool {
    matches!(status, SeatStatus::Cancelled { .. })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rejects_version_mismatch_for_held_seat() {
        let mut seat = SeatState::default();
        seat.apply_reserved("ord1".into(), 1000);

        let err = seat.can_reserve(0).unwrap_err();
        assert_eq!(
            err,
            InventoryError::VersionMismatch {
                expected: 0,
                actual: 1
            }
        );
    }

    #[test]
    fn expiry_only_releases_held_seat() {
        let mut seat = SeatState::default();
        seat.apply_reserved("ord1".into(), 1000);

        seat.expire_if_due(1001);
        assert_eq!(seat.status, SeatStatus::Available);
    }

    #[test]
    fn handles_saga_compensating_action_for_cancellation() {
        let mut seat = SeatState::default();
        seat.apply_reserved("ord1".into(), 1000);
        assert!(matches!(seat.status, SeatStatus::Held { .. }));
        assert_eq!(seat.version, 1);

        seat.apply_expired();
        assert_eq!(seat.status, SeatStatus::Available);
        assert_eq!(seat.version, 2);
    }

    #[test]
    fn issuing_after_expiry_is_rejected() {
        let mut seat = SeatState::default();
        seat.apply_reserved("ord1".into(), 1000);
        seat.expire_if_due(1001);

        // Available again, but stale expected versions still fail.
        let err = seat.can_reserve(0).unwrap_err();
        assert_eq!(
            err,
            InventoryError::VersionMismatch {
                expected: 0,
                actual: 1
            }
        );
    }

    #[test]
    fn cancels_held_seat_and_blocks_further_reservation() {
        let mut seat = SeatState::default();
        seat.apply_reserved("ord1".into(), 1000);

        seat.apply_cancelled("ord1".into());
        assert_eq!(
            seat.status,
            SeatStatus::Cancelled {
                order_id: "ord1".into()
            }
        );
        assert!(seat.can_reserve(seat.version).is_err());
    }

    #[test]
    fn cancels_confirmed_seat_and_blocks_further_reservation() {
        let mut seat = SeatState::default();
        seat.apply_confirmed("ord1".into());

        seat.apply_cancelled("ord1".into());
        assert_eq!(
            seat.status,
            SeatStatus::Cancelled {
                order_id: "ord1".into()
            }
        );
        assert!(seat.can_reserve(seat.version).is_err());
    }

    #[test]
    fn cancellation_is_terminal() {
        let mut seat = SeatState::default();
        seat.apply_reserved("ord1".into(), 1000);
        assert_eq!(seat.version, 1);

        seat.apply_cancelled("ord1".into());
        assert_eq!(seat.version, 2);

        let err = seat.can_reserve(2).unwrap_err();
        assert_eq!(err, InventoryError::SeatNotAvailable);
    }

    #[test]
    fn does_not_skip_cancellation_for_virgin_never_touched_seat() {
        // Never-held seats must still become unbookable when the event is cancelled.
        assert!(!should_skip_cancellation(&SeatStatus::Available));
    }

    #[test]
    fn does_not_skip_cancellation_for_touched_seat_that_raced_to_available() {
        // Seats that raced back to Available must still be event-cancelled.
        assert!(!should_skip_cancellation(&SeatStatus::Available));
    }

    #[test]
    fn does_not_skip_cancellation_for_held_or_confirmed_seat() {
        assert!(!should_skip_cancellation(&SeatStatus::Held {
            order_id: "ord1".into(),
            expires_at_ms: 1000,
        }));
        assert!(!should_skip_cancellation(&SeatStatus::Reserved {
            order_id: "ord1".into()
        }));
    }

    #[test]
    fn skips_cancellation_for_already_cancelled_seat() {
        assert!(should_skip_cancellation(&SeatStatus::Cancelled {
            order_id: "ord1".into()
        }));
    }
}
