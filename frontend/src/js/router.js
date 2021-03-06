//引入vue
import Vue from 'vue';
//引入vue-router
import VueRouter from 'vue-router';
//第三方库需要use一下才能用
Vue.use(VueRouter)
import login from '../components/Login.vue';
import mainPage from '../components/MainPage.vue';

import MapWidget from "../components/MapWidget.vue";
import Devices from "../components/Devices.vue";
import GpsTable from "../components/GpsTable.vue";
import UserManager from "../components/UserManager.vue";
import UserAdd from "../components/UserAdd.vue"

//定义routes路由的集合，数组类型
const routes = [
    //单个路由均为对象类型，path代表的是路径，component代表组件
    { path: '/', redirect: '/login' },
    { path: '/login', component: login },
    {
        path: "/mainpage", component: mainPage,
        children: [
            { path: 'devices', name: 'devices', component: Devices },
            { path: 'gpstable', name: 'gpstable', component: GpsTable },
            { path: 'mapwidget', name: 'mapwidget', component: MapWidget },
            { path: 'userManager', name: 'userManager', component: UserManager },
            { path: 'userAdd', name: 'userAdd', component: UserAdd }
        ]
    }
]

//实例化VueRouter并将routes添加进去
const router = new VueRouter({
    //HTML5 History 模式
    mode: 'history',
    //ES6简写，等于routes：routes
    routes
});

//抛出这个这个实例对象方便外部读取以及访问
export default router