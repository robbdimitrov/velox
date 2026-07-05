use hmac::{Hmac, Mac};
use sha2::Sha256;

type HmacSha256 = Hmac<Sha256>;

/// Falls back to a fixed dev key so local/dev environments keep working without
/// extra setup; production deployments must set EVENT_SIGNING_KEY via a secret store.
pub fn signing_key() -> Vec<u8> {
    std::env::var("EVENT_SIGNING_KEY")
        .unwrap_or_else(|_| "velox-dev-signing-key".to_string())
        .into_bytes()
}

fn mac_for(key: &[u8]) -> HmacSha256 {
    HmacSha256::new_from_slice(key).expect("HMAC accepts any key length")
}

fn canonical(
    mac: &mut HmacSha256,
    event_type: &str,
    aggregate_id: &str,
    aggregate_version: u64,
    payload: &[u8],
) {
    mac.update(event_type.as_bytes());
    mac.update(b"|");
    mac.update(aggregate_id.as_bytes());
    mac.update(b"|");
    mac.update(aggregate_version.to_string().as_bytes());
    mac.update(b"|");
    mac.update(payload);
}

pub fn sign(
    key: &[u8],
    event_type: &str,
    aggregate_id: &str,
    aggregate_version: u64,
    payload: &[u8],
) -> Vec<u8> {
    let mut mac = mac_for(key);
    canonical(
        &mut mac,
        event_type,
        aggregate_id,
        aggregate_version,
        payload,
    );
    mac.finalize().into_bytes().to_vec()
}

pub fn verify(
    key: &[u8],
    event_type: &str,
    aggregate_id: &str,
    aggregate_version: u64,
    payload: &[u8],
    signature: &[u8],
) -> bool {
    let mut mac = mac_for(key);
    canonical(
        &mut mac,
        event_type,
        aggregate_id,
        aggregate_version,
        payload,
    );
    mac.verify_slice(signature).is_ok()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn verifies_matching_signature() {
        let key = b"test-key".to_vec();
        let sig = sign(&key, "SeatReservationHeld", "seat:evt:A:12", 1, b"{}");
        assert!(verify(
            &key,
            "SeatReservationHeld",
            "seat:evt:A:12",
            1,
            b"{}",
            &sig
        ));
    }

    #[test]
    fn rejects_tampered_payload() {
        let key = b"test-key".to_vec();
        let sig = sign(&key, "SeatReservationHeld", "seat:evt:A:12", 1, b"{}");
        assert!(!verify(
            &key,
            "SeatReservationHeld",
            "seat:evt:A:12",
            1,
            b"{\"x\":1}",
            &sig
        ));
    }
}
