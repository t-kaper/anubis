package providers

import "fmt"

func SenderSms(s string) (string, string, error) {
	//TODO.MD: написать отправку sms
	//TODO.MD: написать обработку что потдерживаем покачто только +7 (россию)
	//TODO.MD: понять как можно сделать защиту от большого количества отправки смс
	fmt.Printf("Send sms %s \n", s)
	return s + "IdSend", "mySms_SmsService", nil

}
