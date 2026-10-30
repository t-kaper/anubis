package helpers

//func FillPhoneReg(phone *models.MdAuth, input *schemesAuth.PhoneUserRegValid) error {
//	number, err := strconv.ParseInt(input.Phone, 10, 64)
//	if err != nil {
//		return err
//	}
//	num, err := phonenumbers.Parse(input.Phone, "")
//	if err != nil {
//		return err
//	}
//	phone.Phone = number
//	phone.CountryCode = num.GetCountryCode()
//	phone.PhonePasswordHash, _ = utils.GeneratePasswordHash(input.Password)
//	phone.PhoneVerification = false
//	return nil
//}
