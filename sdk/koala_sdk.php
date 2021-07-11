<?php
/**
 * Koala Rule Engine SDK
 *
 * @package: main
 * @desc: koala engine - php sdk code
 *
 * @author: heiyeluren 
 * @github: https://github.com/heiyeluren
 * @blog: https://blog.csdn.net/heiyeshuwu
 *
 */

/**
 * @概述：此sdk，一方面，实现对koala频率控制服务http接口的简单封装，方便用户调用，
 *        另一方面，我们也推荐您直接调用http接口，sdk给出了接口的请求示例，供参考。
 * 
 * @注意：使用时，请自行在construct函数中传入koala服务的 “ host:port ”，本sdk不维护server地址配置。
 */

class KoalaSdk
{
    // 请求koala服务的地址，host:port
    private $server = '';
    const CURL_TIMEOIT = 3; 

    public function __construct()
    {/*{{{*/
        // 对server地址进行赋值,,重要，，必须！！
        $this->server = "127.0.0.1:6808";
    }/*}}}*/

    /* check *
     * 用途：   检验 是否通过“频率控制检查”，若未通过，给出命中的 “规则id”
     * 接口url：      GET /rule/browse
     * 接口参数：    所有用于匹配规则的 key=value
     *
     * @ 输入参数：      1、包含 所有用户匹配规则的“ K=>V ”的数组
     *                   2、“直接写入”开关（true/false）
     * @ 参数例：        array('action' => 'submit', 'qid' => 123123, 'ip' => '10.16.1.1')
     * @ 返回值：
     *         errno      错误码
     *         errmsg     错误提示
     *         code       若命中某规则，表示命中的“规则号”
     *         vcode_len  若需要出验证码，表示验证码级别
     */
    public function check($param, $writeThrough=false)
    {/*{{{*/
        foreach ($param as $key => &$value) {
            $value = (string)$value;
        }
        if($writeThrough === true)
        {
            $param['_writeThrough'] = 'yes';
        }
        $url = $this->server."/rule/browse";
        $koala_res = $this->send($url, $param);
        $result = array(
            'errno' => $koala_res['Err_no'],
            'errmsg'=> $koala_res['Str_reason'],
            'code'=> $koala_res['Ret_code'],
            'vcode_len' => $koala_res['Vcode_len'],
            );
        return $result;
    }/*}}}*/

    /* check_complete *
     * 用途：   基本功能同check方法，但可能命中多条规则，并返回多个规则id
     * 接口url：      GET /rule/browse_complete
     * 接口参数：    所有用于匹配规则的 key=value
     *
     * @ 输入参数：   同check方法
     * @ 返回值：   类似check方法，但是一个多重数组，对应若干个命中的规则
     */
    public function check_complete($param)
    {/*{{{*/
        foreach ($param as $key => &$value) {
            $value = (string)$value;
        }
        $url = $this->server."/rule/browse_complete";
        $koala_res = $this->send($url, $param);
        if(empty($koala_res))
        {
            return false;
        }
        $result = array();
        foreach ($koala_res as $index => $value) {
            $one_res = array(
                'errno' => $value['Err_no'],
                'errmsg'=> $value['Str_reason'],
                'code'=> $value['Ret_code'],
                'vcode_len' => $value['Vcode_len'],
                );
            $result[$index] = $one_res;
        }
        return $result;
    }/*}}}*/

    /* multiCheck *
     * 用途：   批量check方法，同时发送多个check任务给koala，返回多个结果
     * 接口url：      GET /multi/browse
     * 接口参数：     argsJson （单个任务做urlencode，外层做json_encode，参考方法内部处理)
     *
     * @ 输入参数：      包含多个check任务“key、value对”的两重数组（外层索引数组、内层关联数组）
     * @ 参数例：        array(
     *                      array('action' => 'submit', 'qid' => 123123, 'ip' => '10.16.1.1'),
     *                      array('action' => 'submit', 'qid' => 456456, 'ip' => '10.16.1.1'),
     *                   );
     * @ 返回值：     类似check方法，但是一个多重数组，对应多个check任务的结果
     */
    public function multiCheck($jobParam)
    {/*{{{*/
        $url = $this->server."/multi/browse";
        $jobs = array();
        foreach ($jobParam as $index => $item) {
            foreach ($item as $key => &$value) {
                $value = (string)$value;
            }
            $jobs[$index]['Id'] = (string)$index;
            $jobs[$index]['Arg'] = urlencode(http_build_query($item));
        }
        $jobs = array_merge($jobs);
        $koala_res = $this->send($url, array('argsJson' => json_encode($jobs)));
        $retArray = array();
        foreach ($koala_res as $item) {
            $id = (int)$item['Id'];
            $retArray[$id]['errno'] = $item['Result']['Err_no'];
            $retArray[$id]['errmsg'] = $item['Result']['Str_reason'];
            $retArray[$id]['code'] = $item['Result']['Ret_code'];
            $retArray[$id]['vcode_len'] = $item['Result']['Vcode_len'];
        }
        return $retArray;
    }/*}}}*/

    /* write *
     * 用途：   （必须在调用了check方法之后，才能调用）写入操作，会将所有匹配的规则的缓存值+1
     * 接口url：      GET /rule/update
     * 接口参数：    （必须与check方法的参数一致）所有用于匹配规则的 key=value
     *
     * @ 输入参数：       包含 所有用户匹配规则的“ K=>V ”的数组
     * @ 参数例：        array('action' => 'submit', 'qid' => 123123, 'ip' => '10.16.1.1')
     * @ 返回值：
     *         errno      错误码
     *         errmsg     错误提示
     *         code       若命中某规则，表示命中的“规则号”
     *         vcode_len  若需要出验证码，表示验证码级别
     */
    public function write($param)
    {/*{{{*/
        foreach ($param as $key => &$value) {
            $value = (string)$value;
        }
        $url = $this->server."/rule/update";
        $koala_res = $this->send($url, $param);
        $result = array(
            'errno' => $koala_res['err_no'],
            'errmsg'=> $koala_res['err_msg'],
            );
        return $result;
    }/*}}}*/

    /* monitorAlive *
     * 用途：   koala服务的监控接口，能保证 ： koala存活，以及koala与redis直接的 “连通性”
     * 接口url：      GET /monitor/alive
     *
     * @ 返回值：
     *         errno      错误码
     *         errmsg     错误提示
     */
    public function monitorAlive()
    {/*{{{*/
        $url = $this->server."/monitor/alive";
        $koala_res = $this->send($url);
        return $koala_res;
    }/*}}}*/

    private function send($url, $param=array())
    {/*{{{*/
        $ch = curl_init();
        curl_setopt($ch,CURLOPT_URL, $url.'?'.http_build_query($param));
        curl_setopt($ch,CURLOPT_TIMEOUT,self::CURL_TIMEOIT);
        curl_setopt($ch,CURLOPT_RETURNTRANSFER,true);
        curl_setopt($ch,CURLOPT_HTTPHEADER,array("Host:".$this->_host));
        $rs = curl_exec($ch);
        curl_close($ch);
 
        return json_decode($rs,true);
    }/*}}}*/

}
