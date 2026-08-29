package notify

import (
	"fmt"

	webpush "github.com/SherClockHolmes/webpush-go"

	"github.com/Laaaaksh/escalight/internal/db"
)

// EnsureVAPIDKeys returns the server's VAPID keypair, generating and
// persisting one on first boot if none exists yet in config or the database.
// Keys must stay stable across restarts, or every browser's existing push
// subscription silently stops delivering.
func EnsureVAPIDKeys(store *db.Store, configuredPub, configuredPriv string) (pub, priv string, err error) {
	if configuredPub != "" && configuredPriv != "" {
		return configuredPub, configuredPriv, nil
	}

	if p, ok, err := store.GetSetting("vapid_public_key"); err != nil {
		return "", "", err
	} else if ok {
		s, _, err := store.GetSetting("vapid_private_key")
		if err != nil {
			return "", "", err
		}
		return p, s, nil
	}

	priv, pub, err = webpush.GenerateVAPIDKeys()
	if err != nil {
		return "", "", fmt.Errorf("generate VAPID keys: %w", err)
	}
	if err := store.SetSetting("vapid_public_key", pub); err != nil {
		return "", "", err
	}
	if err := store.SetSetting("vapid_private_key", priv); err != nil {
		return "", "", err
	}
	return pub, priv, nil
}

type WebPushSender struct {
	PublicKey  string
	PrivateKey string
	Subject    string
}

// Send delivers a push message to one subscription. A 404/410 response means
// the browser subscription is gone (uninstalled, cleared site data); callers
// should treat that as a signal to delete the subscription, not a delivery
// failure worth alerting on.
func (w *WebPushSender) Send(sub *db.PushSubscription, payload []byte) (gone bool, err error) {
	resp, err := webpush.SendNotification(payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256dh,
			Auth:   sub.Auth,
		},
	}, &webpush.Options{
		Subscriber:      w.Subject,
		VAPIDPublicKey:  w.PublicKey,
		VAPIDPrivateKey: w.PrivateKey,
		TTL:             300,
		Urgency:         webpush.UrgencyHigh,
	})
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 || resp.StatusCode == 410 {
		return true, nil
	}
	if resp.StatusCode >= 300 {
		return false, fmt.Errorf("push endpoint returned %d", resp.StatusCode)
	}
	return false, nil
}
