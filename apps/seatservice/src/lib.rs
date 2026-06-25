use std::collections::{HashMap, HashSet};
use std::time::{Duration, SystemTime};

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SeatStatus {
    Available,
    Held {
        order_id: String,
        expires_at_ms: u128,
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

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum InventoryError {
    DuplicateEvent,
    VersionMismatch { expected: u64, actual: u64 },
    SeatNotAvailable,
    HoldExpired,
}

#[derive(Default)]
pub struct Inventory {
    seats: HashMap<String, SeatState>,
    processed_events: HashSet<String>,
}

impl Inventory {
    pub fn hold(
        &mut self,
        event_id: &str,
        seat_key: &str,
        order_id: &str,
        expected_version: u64,
        ttl: Duration,
        now: SystemTime,
    ) -> Result<SeatState, InventoryError> {
        if !self.processed_events.insert(event_id.to_string()) {
            return Err(InventoryError::DuplicateEvent);
        }
        self.expire_if_due(seat_key, now);
        let current = self.seats.entry(seat_key.to_string()).or_insert(SeatState {
            status: SeatStatus::Available,
            version: 0,
        });
        if current.version != expected_version {
            return Err(InventoryError::VersionMismatch {
                expected: expected_version,
                actual: current.version,
            });
        }
        if current.status != SeatStatus::Available {
            return Err(InventoryError::SeatNotAvailable);
        }
        let expires_at_ms = now
            .checked_add(ttl)
            .expect("reservation ttl overflow")
            .duration_since(SystemTime::UNIX_EPOCH)
            .expect("time before unix epoch")
            .as_millis();
        current.status = SeatStatus::Held {
            order_id: order_id.to_string(),
            expires_at_ms,
        };
        current.version += 1;
        Ok(current.clone())
    }

    pub fn issue(
        &mut self,
        event_id: &str,
        seat_key: &str,
        order_id: &str,
        expected_version: u64,
        now: SystemTime,
    ) -> Result<SeatState, InventoryError> {
        if !self.processed_events.insert(event_id.to_string()) {
            return Err(InventoryError::DuplicateEvent);
        }
        self.expire_if_due(seat_key, now);
        let current = self.seats.entry(seat_key.to_string()).or_insert(SeatState {
            status: SeatStatus::Available,
            version: 0,
        });
        if current.version != expected_version {
            return Err(InventoryError::VersionMismatch {
                expected: expected_version,
                actual: current.version,
            });
        }
        match &current.status {
            SeatStatus::Held {
                order_id: held_by, ..
            } if held_by == order_id => {
                current.status = SeatStatus::Sold {
                    order_id: order_id.to_string(),
                };
                current.version += 1;
                Ok(current.clone())
            }
            SeatStatus::Available => Err(InventoryError::HoldExpired),
            _ => Err(InventoryError::SeatNotAvailable),
        }
    }

    pub fn expire_if_due(&mut self, seat_key: &str, now: SystemTime) -> Option<SeatState> {
        let current = self.seats.get_mut(seat_key)?;
        let now_ms = now.duration_since(SystemTime::UNIX_EPOCH).ok()?.as_millis();
        if matches!(&current.status, SeatStatus::Held { expires_at_ms, .. } if now_ms >= *expires_at_ms)
        {
            current.status = SeatStatus::Available;
            current.version += 1;
            return Some(current.clone());
        }
        None
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rejects_version_mismatch_for_held_seat() {
        let mut inventory = Inventory::default();
        let now = SystemTime::UNIX_EPOCH + Duration::from_secs(100);
        inventory
            .hold(
                "evt1",
                "event:A:A-01",
                "ord1",
                0,
                Duration::from_secs(300),
                now,
            )
            .unwrap();
        let err = inventory
            .hold(
                "evt2",
                "event:A:A-01",
                "ord2",
                0,
                Duration::from_secs(300),
                now,
            )
            .unwrap_err();
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
        let mut inventory = Inventory::default();
        let now = SystemTime::UNIX_EPOCH + Duration::from_secs(100);
        inventory
            .hold(
                "evt1",
                "event:A:A-02",
                "ord1",
                0,
                Duration::from_secs(1),
                now,
            )
            .unwrap();
        let expired = inventory
            .expire_if_due("event:A:A-02", now + Duration::from_secs(2))
            .unwrap();
        assert_eq!(expired.status, SeatStatus::Available);
    }

    #[test]
    fn issuing_after_expiry_is_rejected() {
        let mut inventory = Inventory::default();
        let now = SystemTime::UNIX_EPOCH + Duration::from_secs(100);
        inventory
            .hold(
                "evt1",
                "event:A:A-03",
                "ord1",
                0,
                Duration::from_secs(1),
                now,
            )
            .unwrap();
        let err = inventory
            .issue(
                "evt2",
                "event:A:A-03",
                "ord1",
                2,
                now + Duration::from_secs(2),
            )
            .unwrap_err();
        assert_eq!(err, InventoryError::HoldExpired);
    }
}
