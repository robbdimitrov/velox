#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SeatStatus {
    Available,
    Held {
        order_id: String,
        expires_at_ms: i64,
    },
    Sold {
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
                // Note: we don't increment version here because expiry is a passive state change
                // based on time. If we wanted an explicit "SeatExpired" event, we would do that
                // at the application layer.
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

    pub fn apply_sold(&mut self, order_id: String) {
        self.status = SeatStatus::Sold { order_id };
        self.version += 1;
    }

    pub fn apply_expired(&mut self) {
        self.status = SeatStatus::Available;
        self.version += 1;
    }

    // Additional domain logic to check if reservation is possible
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

        // At this point it's available, but version is 1. If we try to reserve with version 0:
        let err = seat.can_reserve(0).unwrap_err();
        assert_eq!(
            err,
            InventoryError::VersionMismatch {
                expected: 0,
                actual: 1
            }
        );
    }
}
