package rules
crypTest (x) =y{
    y:={"sha256":crypto.sha256(x), "md5":crypto.md5(x)}
}
tester (x)=y{
  y:=abs(x)
}
encode(x)=y{
    y:={"b64":base64.encode(x), "hex":hex.encode(x)}
}
response:={"crypTest":crypTest(input.str),"test":tester(input.num),"encode":encode(input.str)}

